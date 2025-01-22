package unidler

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/uselagoon/aergia-controller/internal/handlers/metrics"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (h *Unidler) ingressHandler(path string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		opLog := h.Log.WithValues("custom-default-backend", "request")
		start := time.Now()
		ext := "html"
		// if debug is enabled, then set the headers in the response too
		if os.Getenv("DEBUG") == "true" {
			w.Header().Set(FormatHeader, r.Header.Get(FormatHeader))
			w.Header().Set(CodeHeader, r.Header.Get(CodeHeader))
			w.Header().Set(ContentType, r.Header.Get(ContentType))
			w.Header().Set(OriginalURI, r.Header.Get(OriginalURI))
			w.Header().Set(Namespace, r.Header.Get(Namespace))
			w.Header().Set(IngressName, r.Header.Get(IngressName))
			w.Header().Set(ServiceName, r.Header.Get(ServiceName))
			w.Header().Set(ServicePort, r.Header.Get(ServicePort))
			w.Header().Set(RequestID, r.Header.Get(RequestID))
		}
		format := r.Header.Get(FormatHeader)
		if format == "" {
			format = "text/html"
		}

		cext, err := mime.ExtensionsByType(format)
		if err != nil {
			// log.Printf("unexpected error reading media type extension: %v. Using %v", err, ext)
			format = "text/html"
		} else if len(cext) == 0 {
			// log.Printf("couldn't get media type extension. Using %v", ext)
		} else {
			ext = cext[0]
		}
		w.Header().Set(ContentType, format)
		w.Header().Set(AergiaHeader, "true")
		w.Header().Set(CacheControl, "private,no-store")

		errCode := r.Header.Get(CodeHeader)
		code, err := strconv.Atoi(errCode)
		if err != nil {
			code = 404
		}
		w.WriteHeader(code)
		ns := r.Header.Get(Namespace)
		ingressName := r.Header.Get(IngressName)
		// check if the namespace exists so we know this is somewhat legitimate request
		if ns != "" {
			namespace := &corev1.Namespace{}
			if err := h.Client.Get(ctx, types.NamespacedName{
				Name: ns,
			}, namespace); err != nil {
				opLog.Info(fmt.Sprintf("unable to get any namespaces: %v", err))
				return
			}
			ingress := &networkv1.Ingress{}
			if err := h.Client.Get(ctx, types.NamespacedName{
				Namespace: ns,
				Name:      ingressName,
			}, ingress); err != nil {
				opLog.Info(fmt.Sprintf("Unable to get the ingress %s in %s", ingressName, ns))
				h.genericError(w, r, opLog, ext, format, path, 400)
				h.setMetrics(r, start)
				return
			}
			// if hmac verification is enabled, perform the verification of the request
			signedNamespace, verfied := h.verifyRequest(r, namespace, ingress)

			xForwardedFor := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
			trueClientIP := r.Header.Get("True-Client-IP")
			requestUserAgent := r.Header.Get("User-Agent")

			allowUnidle := h.checkAccess(namespace.ObjectMeta.Annotations, ingress.ObjectMeta.Annotations, requestUserAgent, trueClientIP, xForwardedFor)
			// then run checks to start to unidle the environment
			if allowUnidle {
				// if a namespace exists, it means that the custom-http-errors code is defined in the ingress object
				// so do something with that here, like kickstart the idler process to unidle targets
				if h.Debug {
					opLog.Info(fmt.Sprintf("Request for %s verfied: %t from xff:%s; tcip:%s; ua: %s, ", ns, verfied, xForwardedFor, trueClientIP, requestUserAgent))
				}

				file := fmt.Sprintf("%v/unidle.html", path)
				forceScaled := h.checkForceScaled(ctx, ns, opLog)
				if forceScaled {
					// if this has been force scaled, return the force scaled landing page
					file = fmt.Sprintf("%v/forced.html", path)
				} else {
					// only unidle environments that aren't force scaled
					// actually do the unidling here, lock to prevent multiple unidle operations from running
					if verfied {
						if h.Debug {
							opLog.Info(fmt.Sprintf("Request for %s verfied", ns))
						}
						metrics.AllowedRequests.Inc()
						w.Header().Set("X-Aergia-Allowed", "true")
						_, ok := h.Locks.Load(ns)
						if !ok {
							_, _ = h.Locks.LoadOrStore(ns, ns)
							if h.Debug {
								opLog.Info(fmt.Sprintf("Unidle request for %s verfied", ns))
							}
							go h.Unidle(ctx, namespace, opLog)
						}
					} else {
						metrics.VerificationRequired.Inc()
						w.Header().Set("X-Aergia-Verification-Required", "true")
					}
				}
				if h.Debug {
					opLog.Info(fmt.Sprintf("Serving custom error response for code %v and format %v from file %v", code, format, file))
				}
				// then return the unidle template to the user
				tmpl := template.Must(template.ParseFiles(file))
				tmpl.ExecuteTemplate(w, "base", pageData{
					ErrorCode:       strconv.Itoa(code),
					FormatHeader:    r.Header.Get(FormatHeader),
					CodeHeader:      r.Header.Get(CodeHeader),
					ContentType:     r.Header.Get(ContentType),
					OriginalURI:     r.Header.Get(OriginalURI),
					Namespace:       r.Header.Get(Namespace),
					IngressName:     r.Header.Get(IngressName),
					ServiceName:     r.Header.Get(ServiceName),
					ServicePort:     r.Header.Get(ServicePort),
					RequestID:       r.Header.Get(RequestID),
					RefreshInterval: h.RefreshInterval,
					Verifier:        signedNamespace,
				})
			} else {
				// respond with forbidden
				w.Header().Set("X-Aergia-Denied", "true")
				metrics.BlockedRequests.Inc()
				h.genericError(w, r, opLog, ext, format, path, 403)
			}
		} else {
			w.Header().Set("X-Aergia-Denied", "true")
			w.Header().Set("X-Aergia-No-Namespace", "true")
			metrics.NoNamespaceRequests.Inc()
			h.genericError(w, r, opLog, ext, format, path, code)
		}
		h.setMetrics(r, start)
	}
}

func (h *Unidler) genericError(w http.ResponseWriter, r *http.Request, opLog logr.Logger, format, path, verifier string, code int) {
	file := fmt.Sprintf("%v/error.html", path)
	if h.Debug {
		opLog.Info(fmt.Sprintf("Serving custom error response for code %v and format %v from file %v", code, format, file))
	}
	tmpl := template.Must(template.ParseFiles(file))
	tmpl.ExecuteTemplate(w, "base", pageData{
		ErrorCode:       strconv.Itoa(code),
		ErrorMessage:    http.StatusText(code),
		FormatHeader:    r.Header.Get(FormatHeader),
		CodeHeader:      r.Header.Get(CodeHeader),
		ContentType:     r.Header.Get(ContentType),
		OriginalURI:     r.Header.Get(OriginalURI),
		Namespace:       r.Header.Get(Namespace),
		IngressName:     r.Header.Get(IngressName),
		ServiceName:     r.Header.Get(ServiceName),
		ServicePort:     r.Header.Get(ServicePort),
		RequestID:       r.Header.Get(RequestID),
		RefreshInterval: h.RefreshInterval,
		Verifier:        verifier,
	})
}

// handle verifying the namespace name is signed by our secret
func (h *Unidler) verifyRequest(r *http.Request, ns *corev1.Namespace, ingress *networkv1.Ingress) (string, bool) {
	if h.VerifiedUnidling {
		if val, ok := ingress.ObjectMeta.Annotations["idling.amazee.io/disable-request-verification"]; ok {
			t, _ := strconv.ParseBool(val)
			if t {
				return "", true
			}
			// otherwise fall through to namespace check
		}
		if val, ok := ns.ObjectMeta.Annotations["idling.amazee.io/disable-request-verification"]; ok {
			t, _ := strconv.ParseBool(val)
			if t {
				return "", true
			}
			// fall through to verify the request
		}
		// if hmac verification is enabled, perform the verification of the request
		signedNamespace := hmacSigner(ns.Name, []byte(h.VerifiedSecret))
		verifier := r.URL.Query().Get("verifier")
		metrics.VerificationRequests.Inc()
		return signedNamespace, hmacVerifier(ns.Name, verifier, []byte(h.VerifiedSecret))
	}
	return "", true
}

func (h *Unidler) setMetrics(r *http.Request, start time.Time) {
	duration := time.Since(start).Seconds()

	proto := strconv.Itoa(r.ProtoMajor)
	proto = fmt.Sprintf("%s.%s", proto, strconv.Itoa(r.ProtoMinor))

	metrics.RequestCount.WithLabelValues(proto).Inc()
	metrics.RequestDuration.WithLabelValues(proto).Observe(duration)
}
