# Traffic Router Plugins

!!! warning "Alpha Feature (Since 1.5.0)"

    This is an experimental, [alpha-quality](https://github.com/argoproj/argoproj/blob/main/community/feature-status.md#alpha)
    feature that allows you to supporttraffic router that are not natively supported.

Argo Rollouts supports getting traffic router via 3rd party [plugin system](../../plugins.md). This allows users to extend the capabilities of Rollouts
to support traffic router that are not natively supported. Rollout's uses a plugin library called
[go-plugin](https://github.com/hashicorp/go-plugin) to do this. You can find a sample plugin
here: [rollouts-plugin-trafficrouter-sample-nginx](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-sample-nginx)

## Installing

There are two methods of installing and using an argo rollouts plugin. The first method is to mount up the plugin executable
into the rollouts controller container. The second method is to use an HTTP(S) server to host the plugin executable.

### Mounting the plugin executable into the rollouts controller container

There are a few ways to mount the plugin executable into the rollouts controller container. Some of these will depend on your
particular infrastructure. Here are a few methods:

- Using an init container to download the plugin executable
- Using a Kubernetes volume mount with a shared volume such as NFS, EBS, etc.
- Building the plugin into the rollouts controller container

Then you can use the configmap to point to the plugin executable file location. Example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/sample-nginx" # name of the plugin, it must match the name required by the plugin so it can find its configuration
      location: "file://./my-custom-plugin" # supports http(s):// urls and file://
```

### Using an HTTP(S) server to host the plugin executable

!!! warning "Installing a plugin with http(s)"

    Depending on which method you use to install and the plugin, there are some things to be aware of.
    The rollouts controller will not start if it can not download or find the plugin executable. This means that if you are using
    a method of installation that requires a download of the plugin and the server hosting the plugin for some reason is not available and the rollouts
    controllers pod got deleted while the server was down or is coming up for the first time, it will not be able to start until
    the server hosting the plugin is available again.

    Argo Rollouts will download the plugin at startup only once but if the pod is deleted it will need to download the plugin again on next startup. Running
    Argo Rollouts in HA mode can help a little with this situation because each pod will download the plugin at startup. So if a single pod gets
    deleted during a server outage, the other pods will still be able to take over because there will already be a plugin executable available to it. It is the
    responsibility of the Argo Rollouts administrator to define the plugin installation method considering the risks of each approach.

Argo Rollouts supports downloading the plugin executable from an HTTP(S) server. To use this method, you will need to
configure the controller via the `argo-rollouts-config` configmap and set `pluginLocation` to a http(s) url. Example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/sample-nginx" # name of the plugin, it must match the name required by the plugin so it can find its configuration
      location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-sample-nginx/releases/download/v0.0.1/metric-plugin-linux-amd64" # supports http(s):// urls and file://
      sha256: "08f588b1c799a37bbe8d0fc74cc1b1492dd70b2c" #optional sha256 checksum of the plugin executable
      headersFrom: #optional headers for the download via http request 
        - secretRef:
            name: secret-name
---
apiVersion: v1
kind: Secret
metadata:
  name: secret-name
stringData:
  Authorization: Basic <Base 64 TOKEN>
  My-Header: value
```

## List of Available Plugins (alphabetical order)

If you have created a plugin, please submit a PR to add it to this list.

### [rollouts-plugin-trafficrouter-sample-nginx](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-sample-nginx)

- This is just a sample plugin that can be used as a starting point for creating your own plugin.
  It is not meant to be used in production. It is based on the built-in prometheus provider.

### [Consul](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-consul)

- This is a plugin that allows argo-rollouts to work with Consul's service mesh for traffic shaping patterns.

### [Contour](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour)

- This is a plugin that allows argo-rollouts to work with contour's resource: HTTPProxy. It enables traffic shaping patterns such as canary releases and more.

### [Gateway API](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/)

- Provide support for Gateway API, which includes Kuma, Traefix, cilium, Contour, GloodMesh, HAProxy, and [many others](https://gateway-api.sigs.k8s.io/implementations/#implementation-status).
