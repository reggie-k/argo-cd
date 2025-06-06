# Templates

The template fields of the ApplicationSet `spec` are used to generate Argo CD `Application` resources.

ApplicationSet is using [fasttemplate](https://github.com/valyala/fasttemplate) but will be soon deprecated in favor of Go Template. 

## Template fields

An Argo CD Application is created by combining the parameters from the generator with fields of the template (via `{{values}}`), and from that a concrete `Application` resource is produced and applied to the cluster.

Here is the template subfield from a Cluster generator:

```yaml
# (...)
 template:
   metadata:
     name: '{{ .nameNormalized }}-guestbook'
   spec:
     source:
       repoURL: https://github.com/infra-team/cluster-deployments.git
       targetRevision: HEAD
       path: guestbook/{{ .nameNormalized }}
     destination:
       server: '{{ .server }}'
       namespace: guestbook
```

For details on all available parameters (like `.name`, `.nameNormalized`, etc.) please refer to the [Cluster Generator docs](./Generators-Cluster.md).

The template subfields correspond directly to [the spec of an Argo CD `Application` resource](../../declarative-setup/#applications):

- `project` refers to the [Argo CD Project](../../user-guide/projects.md) in use (`default` may be used here to utilize the default Argo CD Project)
- `source` defines from which Git repository to extract the desired Application manifests
    - **repoURL**: URL of the repository (eg `https://github.com/argoproj/argocd-example-apps.git`)
    - **targetRevision**: Revision (tag/branch/commit) of the repository (eg `HEAD`)
    - **path**: Path within the repository where Kubernetes manifests (and/or Helm, Kustomize, Jsonnet resources) are located
- `destination`: Defines which Kubernetes cluster/namespace to deploy to
    - **name**: Name of the cluster (within Argo CD) to deploy to
    - **server**: API Server URL for the cluster (Example: `https://kubernetes.default.svc`)
    - **namespace**: Target namespace in which to deploy the manifests from `source` (Example: `my-app-namespace`)

Note:

- Referenced clusters must already be defined in Argo CD, for the ApplicationSet controller to use them
- Only **one** of `name` or `server` may be specified: if both are specified, an error is returned.
- Signature Verification does not work with the templated `project` field when using git generator.

The `metadata` field of template may also be used to set an Application `name`, or to add labels or annotations to the Application.

While the ApplicationSet spec provides a basic form of templating, it is not intended to replace the full-fledged configuration management capabilities of tools such as Kustomize, Helm, or Jsonnet.

### Deploying ApplicationSet resources as part of a Helm chart

ApplicationSet uses the same templating notation as Helm (`{{}}`). When Helm renders the chart templates, it will also
process the template meant for ApplicationSet rendering. If the ApplicationSet template uses a function like:

```yaml
    metadata:
      name: '{{ "guestbook" | normalize }}'
```

Helm will throw an error like: `function "normalize" not defined`. If the ApplicationSet template uses a generator parameter like:

```yaml
    metadata:
      name: '{{.cluster}}-guestbook'
```

Helm will silently replace `.cluster` with an empty string.

To avoid those errors, write the template as a Helm string literal. For example:

```yaml
    metadata:
      name: '{{`{{ .cluster | normalize }}`}}-guestbook'
```

This _only_ applies if you use Helm to deploy your ApplicationSet resources.

## Generator templates

In addition to specifying a template within the `.spec.template` of the `ApplicationSet` resource, templates may also be specified within generators. This is useful for overriding the values of the `spec`-level template.

The generator's `template` field takes precedence over the `spec`'s template fields:

- If both templates contain the same field, the generator's field value will be used.
- If only one of those templates' fields has a value, that value will be used.

Generator templates can thus be thought of as patches against the outer `spec`-level template fields.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  generators:
  - list:
      elements:
        - cluster: engineering-dev
          url: https://kubernetes.default.svc
      template:
        metadata: {}
        spec:
          project: "default"
          source:
            targetRevision: HEAD
            repoURL: https://github.com/argoproj/argo-cd.git
            # New path value is generated here:
            path: 'applicationset/examples/template-override/{{ .nameNormalized }}-override'
          destination: {}

  template:
    metadata:
      name: '{{ .nameNormalized }}-guestbook'
    spec:
      project: "default"
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        # This 'default' value is not used: it is replaced by the generator's template path, above
        path: applicationset/examples/template-override/default
      destination:
        server: '{{ .server }}'
        namespace: guestbook
```
(*The full example can be found [here](https://github.com/argoproj/argo-cd/tree/master/applicationset/examples/template-override).*)

In this example, the ApplicationSet controller will generate an `Application` resource using the `path` generated by the List generator, rather than the `path` value defined in `.spec.template`.

## Template Patch

Templating is only available on string type. However, some use cases may require applying templating on other types.

Example:

- Conditionally set the automated sync policy.
- Conditionally switch prune boolean to `true`.
- Add multiple helm value files from a list.

The `templatePatch` feature enables advanced templating, with support for `json` and `yaml`.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  generators:
  - list:
      elements:
        - cluster: engineering-dev
          url: https://kubernetes.default.svc
          autoSync: true
          prune: true
          valueFiles:
            - values.large.yaml
            - values.debug.yaml
  template:
    metadata:
      name: '{{ .nameNormalized }}-deployment'
    spec:
      project: "default"
      source:
        repoURL: https://github.com/infra-team/cluster-deployments.git
        targetRevision: HEAD
        path: guestbook/{{ .nameNormalized }}
      destination:
        server: '{{ .server }}'
        namespace: guestbook
  templatePatch: |
    spec:
      source:
        helm:
          valueFiles:
          {{- range $valueFile := .valueFiles }}
            - {{ $valueFile }}
          {{- end }}
    {{- if .autoSync }}
      syncPolicy:
        automated:
          prune: {{ .prune }}
    {{- end }}
```

!!! important
    `templatePatch` only works when [go templating](../applicationset/GoTemplate.md) is enabled.
    This means that the `goTemplate` field under `spec` needs to be set to `true` for template patching to work.

!!! important
    The `templatePatch` can apply arbitrary changes to the template. If parameters include untrustworthy user input, it 
    may be possible to inject malicious changes into the template. It is recommended to use `templatePatch` only with 
    trusted input or to carefully escape the input before using it in the template. Piping input to `toJson` should help
    prevent, for example, a user from successfully injecting a string with newlines.

    The `spec.project` field is not supported in `templatePatch`. If you need to change the project, you can use the
    `spec.project` field in the `template` field.

!!! important
    When writing a `templatePatch`, you're crafting a patch. So, if the patch includes an empty `spec: # nothing in here`, it will effectively clear out existing fields. See [#17040](https://github.com/argoproj/argo-cd/issues/17040) for an example of this behavior.
