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
			// log.Printf("format not specified. Using %v", format)
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
			// log.Printf("unexpected error reading return code: %v. Using %v", err, code)
		}
		w.WriteHeader(code)
		ns := r.Header.Get(Namespace)
		ingressName := r.Header.Get(IngressName)
		// @TODO: check for code 503 specifically, or just any request that has the namespace in it will be "unidled" if a request comes in for
		// that ingress and the
		if ns != "" {
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

			xForwardedFor := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
			trueClientIP := r.Header.Get("True-Client-IP")
			requestUserAgent := r.Header.Get("User-Agent")
			fmt.Println(xForwardedFor, trueClientIP, requestUserAgent)

			allowUnidle := h.checkAccess(ingress.ObjectMeta.Annotations, requestUserAgent, trueClientIP, xForwardedFor)
			// then run checks to start to unidle the environment
			if allowUnidle {
				// if a namespace exists, it means that the custom-http-errors code is defined in the ingress object
				// so do something with that here, like kickstart the idler process to unidle targets
				opLog.Info(fmt.Sprintf("Got request in namespace %s", ns))

				file := fmt.Sprintf("%v/unidle.html", path)
				forceScaled := h.checkForceScaled(ctx, ns, opLog)
				if forceScaled {
					// if this has been force scaled, return the force scaled landing page
					file = fmt.Sprintf("%v/forced.html", path)
				} else {
					// only unidle environments that aren't force scaled
					// actually do the unidling here, lock to prevent multiple unidle operations from running
					_, ok := h.Locks.Load(ns)
					if !ok {
						_, _ = h.Locks.LoadOrStore(ns, ns)
						go h.UnIdle(ctx, ns, opLog)
					}
				}
				if h.Debug == true {
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
				})
			} else {
				// respond with 503 to match the standard request
				w.Header().Set("X-Aergia-Blocked", "true")
				h.genericError(w, r, opLog, ext, format, path, 503)
			}
		} else {
			w.Header().Set("X-Aergia-No-Namespace", "true")
			h.genericError(w, r, opLog, ext, format, path, code)
		}
		h.setMetrics(r, start)
	}
}

func (h *Unidler) genericError(w http.ResponseWriter, r *http.Request, opLog logr.Logger, ext, format, path string, code int) {
	// otherwise just handle the generic http responses here
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	file := fmt.Sprintf("%v/error.html", path)
	if h.Debug == true {
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
	})
}
