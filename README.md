# Aergia

> In Greek mythology, Aergia is the personification of sloth, idleness, indolence and laziness

Aergia is a controller that can be used to scale deployments from zero when a request is made to an ingress with a zero scaled deployment.

This controller replaces the ingress-nginx default backend with this custom backend.

This backend is designed to serve generic error handling for any http error. The backend can also leverage [custom errors](https://kubernetes.github.io/ingress-nginx/user-guide/custom-errors/), which can be used to check the kubernetes api to see if the namespace needs to be scaled from zero.

## Change the default templates

By using the environment variable `ERROR_FILES_PATH`, and pointing to a location that contains the three templates `error.html`, `forced.html`, and `unidle.html`, you can change what is shown to the end user.

This could be done using a configmap and volume mount to any directory, then update the `ERROR_FILES_PATH` to this directory.

# Installation

Install via helm (https://github.com/amazeeio/charts/tree/main/charts/aergia)

## Custom templates
If installing via helm, you can use this YAML in your values.yaml file and define the templates there.

> See `www/error.html`, `www/force.html`, and `www/unidle.html` for inspiration

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
    <meta http-equiv="refresh" content="{{ .RefreshInterval }}">
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