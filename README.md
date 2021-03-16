# Aergia

> In Greek mythology, Aergia is the personification of sloth, idleness, indolence and laziness

Aergia is a controller that can be used to scale deployments from zero when a request is made to an ingress with a zero scaled deployment.

This controller replaces the ingress-nginx default backend with this custom backend.

This backend is designed to serve generic error handling for any http error. The backend can also leverage [custom errors](https://kubernetes.github.io/ingress-nginx/user-guide/custom-errors/), which can be used to check the kubernetes api to see if the namespace needs to be scaled from zero.

## Change the default templates

By using the environment variable `ERROR_FILES_PATH`, and pointing to a location that contains the two templates `error.html` and `unidle.html`, you can change what is shown to the end user.

This could be done using a configmap and volume mount to any directory, then update the `ERROR_FILES_PATH` to this directory.

# Installation

Install via helm

Clone this repo then run the following and select the template version
```
./helm-update install-tgz
```

Alternatively, run it manually with a custom values file
```
helm upgrade --install --create-namespace -n aergia aergia charts/aergia-$chartversion.tgz --values values.yaml
```

## Custom templates
If installing via helm, you can use this YAML in your values.yaml file and define the templates there.

> See `www/error.html` and `www/unidle.html` for inspiration

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
```