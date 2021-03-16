#!/bin/bash

# ./helm-update.sh index
#   create new index for the chart

# ./helm-update.sh template
#   process the chart to a template

# ./helm-update.sh delete
#   delete the chart from kubernetes

# ./helm-update.sh install
#   install the chart into kubernetes

# ./helm-update.sh install-tgz
#   install the chart from one of the tgz files present locally into kubernetes

case $1 in
  index)
    pushd charts
    helm package aergia
    helm repo index .
    popd
    ;;
  template)
    helm template charts/aergia -f charts/aergia/values.yaml
    ;;
  delete)
    helm delete -n aergia aergia
    ;;
  install)
    helm repo add aergia https://raw.githubusercontent.com/amazeeio/unidler/main/charts
    helm upgrade --install -n aergia aergia aergia/aergia
    ;;
  install-tgz)
    options=($(ls charts | grep tgz))
    if [ ${#options[@]} -ne 0 ]; then
      select chart in "${options[@]}";
      do
        case $chart in
              "$QUIT")
                echo "Unknown option, exiting."
                break
                ;;
              *)
                break
                ;;
        esac
      done
      if [ "$chart" != "" ]; then
        helm upgrade --install --create-namespace -n aergia aergia charts/$chart
      fi
    else
      echo "No chart files, exiting."
    fi
    ;;
  *)
    echo "nothing"
    ;;
esac

