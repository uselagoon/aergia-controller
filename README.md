# Aergia

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/uselagoon/aergia-controller/badge)](https://securityscorecards.dev/viewer/?uri=github.com/uselagoon/aergia-controller)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/10427/badge)](https://www.bestpractices.dev/projects/10427)
[![coverage](https://raw.githubusercontent.com/uselagoon/aergia-controller/badges/.badges/main/coverage.svg)](https://github.com/uselagoon/aergia-controller/actions/workflows/coverage.yaml)

> In Greek mythology, Aergia is the personification of sloth, idleness, indolence and laziness

Aergia is a controller that can be used to scale deployments from zero when a request is made to an ingress with a zero scaled deployment.

This controller replaces the ingress-nginx default backend with this custom backend.

This backend is designed to serve generic error handling for any http error. The backend can also leverage [custom errors](https://kubernetes.github.io/ingress-nginx/user-guide/custom-errors/), which can be used to check the kubernetes api to see if the namespace needs to be scaled from zero.

## Usage

An environment can be force idled, force scaled, or unidled using labels on the namespace. All actions still respect the label selectors, but forced actions will bypass any hits checks

### Force Idled
To force idle a namespace, you can label the namespace using `idling.lagoon.sh/force-idled=true`. This will cause the environment to be immediately scaled down, but the next request to the ingress in the namespace will unidle the namespace

### Force Scaled
To force scale a namespace, you can label the namespace using `idling.lagoon.sh/force-scaled=true`. This will cause the environment to be immediately scaled down, but the next request to the ingress in the namespace will *NOT* unidle the namespace. A a deployment will be required to unidle this namespace

### Unidle
To unidle a namespace, you can label the namespace using `idling.lagoon.sh/unidle=true`. This will cause the environment to be scaled back up to its previous state.

### Idled
A label `idling.lagoon.sh/idled` is set that will be true or false depending on if the environment is idled. This ideally should not be modified as Aergia will update it as required.

### Namespace Idling Overrides
If you want to change a namespaces interval check times outside of the globally applied intervals, the following annotations can be added to the namespace
* `idling.lagoon.sh/prometheus-interval` - set this to the time interval for prometheus checks, the format must be in [30m|4h|1h30m](https://pkg.go.dev/time#ParseDuration) notation
* `idling.lagoon.sh/pod-interval` - set this to the time interval for pod uptime checks, the format must be in [30m|4h|1h30m](https://pkg.go.dev/time#ParseDuration) notation

### IP Allow/Block Lists
It is possible to add global IP allow and block lists, the helm chart will have support for handling this creation
* allowing IP addresses via `/lists/allowedips` file which is a single line per entry of ip address to allow
* blocking IP addresses via `/lists/blockedips` file which is a single line per entry of ip address to block

There are also annotations that can be added to the namespace, or individual `Kind: Ingress` objects that allow for ip allow or blocking.
* `idling.lagoon.sh/ip-allow-list` - a comma separated list of ip addresses to allow, will be checked against x-forward-for, but if true-client-ip is provided it will prefer this.
* `idling.lagoon.sh/ip-block-list` - a comma separated list of ip addresses to allow, will be checked against x-forward-for, but if true-client-ip is provided it will prefer this.

### UserAgent Allow/Block Lists
It is possible to add global UserAgent allow and block lists, the helm chart will have support for handling this creation
* allowing user agents via a `/lists/allowedagents` file which is a single line per entry of useragents or regex patterns to match against. These must be `go` based regular expressions.
* blocking user agents via a `/lists/blockedagents` file which is a single line per entry of useragents or regex patterns to match against. These must be `go` based regular expressions.

There are also annotations that can be added to the namespace, or individual `Kind: Ingress` objects that allow for user agent allow or blocking.
* `idling.lagoon.sh/allowed-agents` - a comma separated list of user agents or regex patterns to allow.
* `idling.lagoon.sh/blocked-agents` - a comma separated list of user agents or regex patterns to block.

### Verify Unidling Requests
It is possible to start Aergia in a mode where it will require unidling requests to be verified. The way this works is by using HMAC and passing the signed version of the requested namespace back to the user when the initial request to unidle the environment is received. When a client loads this page, it will execute a javascript query back to the requested ingress which is then verified by Aergia. If verification suceeds, it proceeds to unidle the environment. This functionality can be useful to prevent bots and other systems that don't have the ability to execute javascript from unidling environments uncessarily. The signed namespace value will only work for the requested namespace.

To enable this functionality, set the following:
- `--verified-unidling=true` or envvar `VERIFIED_UNIDLING=true`
- `--verify-secret=use-your-own-secret` or envvar `VERIFY_SECRET=use-your-own-secret`

If the verification feature is enabled, and you need to unidle environments using tools that can't execute javascript, then it is possible to allow a namespace to override the feature by adding the following annotation to the namespace. Using the other allow/blocking mechanisms can then be used to restrict how the environment can unidle if required.
* `idling.lagoon.sh/disable-request-verification=true` - set this to disable the hmac verification on a namespace if Aergia has unidling request verification turned on. This annotation is also supported on an ingress too, so that specific ingress can skip the verification requests.

If you're using custom template overrides and enable this functionality, you will need to extend your `unidle.html` template with the additional changes to allow it to to perform the call back function or else environments will never unidle. See the bundled `unidle.html` file to see how this may differ from your custom templates.

## Change the default templates

By using the environment variable `ERROR_FILES_PATH`, and pointing to a location that contains the three templates `error.html`, `forced.html`, and `unidle.html`, you can change what is shown to the end user.

This could be done using a configmap and volume mount to any directory, then update the `ERROR_FILES_PATH` to this directory.

# Installation

Install via lagoon-remote helm chart.

## Custom templates
If installing via helm, you can use this YAML in your values.yaml file and define the templates there.

> See `www/error.html`, `www/forced.html`, and `www/unidle.html` for inspiration

```
templates:
  enabled: false
  error: |
    {{define "base"}}
    <html>
    <body>
    {{ .ErrorCode }} {{ .ErrorMessage }}
    </body>
    </html>
    {{end}}
  unidle: |
    {{define "base"}}
    <html>
    <head>
    <meta http-equiv="refresh" content="{{ .RefreshInterval }}">
    </head>
    <body>
    {{ .ErrorCode }} {{ .ErrorMessage }}
    </body>
    </html>
    {{end}}
  forced: |
    {{define "base"}}
    <html>
    <head>
    </head>
    <body>
    {{ .ErrorCode }} {{ .ErrorMessage }}
    </body>
    </html>
    {{end}}
```

## Prometheus
The idler uses prometheus to check if there has been hits to the ingress in the last defined interval, it only checks status codes of 200.
By default it will talk to a prometheus in cluster `http://monitoring-kube-prometheus-prometheus.monitoring.svc:9090` but this is adjustable with a flag (and via helm values).

### Requirements
One of the requirements of using prometheus is the ability to query for ingress-nginx requests using this metric `nginx_ingress_controller_requests`

You need to ensure that your ingress-nginx controller is scraped for this metric or else the idler will assume there have been 0 hits and idle the environment without hesitation.

An example `ServiceMonitor` is found in this repo under `test-resources/ingress-servicemonitor.yaml`
