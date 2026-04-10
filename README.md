> :warning: This project is currently **under heavy development and is not considered stable yet**. This means that there may be bugs or unexpected behavior, and we don't recommend using it in production.

# Admiral

[![Release](https://img.shields.io/github/v/release/admiral-io/admiral)](https://github.com/admiral-io/admiral/releases/latest)
[![GitHub License](https://img.shields.io/github/license/admiral-io/admiral)](https://github.com/admiral-io/admiral/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/admiral-io/admiral)](https://goreportcard.com/report/github.com/admiral-io/admiral)

**Deploy with confidence.** Admiral is an open-source platform orchestrator. Your IaC provisions infrastructure. Your manifests deploy apps. Admiral maintains the dependency graph across both so config flows automatically, environments stay consistent, and every deployment is a snapshot you can roll back.

## What is Admiral?

Admiral sits between your IaC and your deployment tooling, maintains the dependency graph across both, and gives you an application-centric model that reduces environment duplication.

## Why Admiral?

### Infra ↔ Workload Glue
Admiral lets your workloads reference IaC outputs directly. Connection strings, ARNs, and endpoints flow from infrastructure to deployment without manual handoff.

### Application-Centric
Define your app once, layer environment-specific config on top. Dev, staging, and production share the same model. Only the values differ, not the manifests.

### Works With Your Toolchain
A CLI for scripting and CI/CD, a GitHub Action for deploy-on-merge workflows, and a Terraform provider. Admiral orchestrates the tools you already use (Helm, Kustomize, Terraform, OpenTofu) without asking you to adopt a new workload spec.

### GitOps Without the Git
Declarative desired state, immutable versioned snapshots, pull-based agents, continuous reconciliation. Built on the same principles as the CNCF OpenGitOps project, without managing Git repositories as the state store.

### OpenAPI Spec
Fully documented OpenAPI spec with the ability to generate clients in any language. Everything the CLI and web UI can do is available to your automation.

### No Lock-In
Admiral orchestrates your tools but doesn't replace them. Your Helm charts, Kustomize overlays, and Terraform modules stay in standard formats. Stop using Admiral and you still have all your work.

## Documentation

See [admiral.io/docs](https://admiral.io/docs) for official documentation.

## Contributing

Found a bug or have an idea? We'd love to hear from you! Open a [GitHub issue](https://github.com/admiral-io/admiral/issues/new/choose) to get started. Check out our [good first issues](https://github.com/admiral-io/admiral/labels/contributor-program) if you're looking for a place to contribute. See our [Code of Conduct](https://github.com/admiral-io/admiral/tree/master/.github/CODE_OF_CONDUCT.md) to learn about our community standards.

If you're setting up a local development environment, see the [Development Setup](docs/DEV-SETUP.md) guide.

## Security

We take security issues very seriously. **Please do not file GitHub issues or post on our public forum for security vulnerabilities**. Vulnerabilies can be disclosed in private using [GitHub advisories](https://github.com/admiral-io/admiral/security) if you believe you have uncovered a vulnerability. In the message, try to provide a description of the issue and ideally a way of reproducing it. The security team will get back to you as soon as possible.

## License

See the [LICENSE](https://github.com/admiral-io/admiral/tree/master/LICENSE) file for licensing information

## Thank You

Admiral would not be possible without the support and assistance of other open-source tools and companies.

<a href="https://github.com/admiral-io/admiral/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=admiral-io/admiral" alt="Admiral Contributors"/>
</a>
