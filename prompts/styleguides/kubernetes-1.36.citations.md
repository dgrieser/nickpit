### Citations: Kubernetes 1.36: Manifests, Policies and Go Controllers — Complete Review Guide

Footnote sources for `kubernetes-1.36.md`. Not sent to the model; kept for maintainers.

[^1]: Kubernetes v1.36: ハル (Haru) — release announcement (2026-04-22), including graduations, deprecations and the Ingress NGINX note. https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/
[^2]: Kubernetes releases and patch-release schedule (1.36 maintenance mode 2027-04-28, EOL 2027-06-28; 1.37.0 on 2026-08-26). https://kubernetes.io/releases/ , https://kubernetes.io/releases/patch-releases/ ; schedule data: https://github.com/kubernetes/website/blob/main/data/releases/schedule.yaml ; current patch: https://dl.k8s.io/release/stable-1.36.txt
[^3]: CHANGELOG-1.36 (v1.36.0 … v1.36.5). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md
[^4]: controller-runtime releases v0.24.0 / v0.24.1 (k8s.io/* v0.36, Go 1.26) and v0.23.0 (typed webhooks, events API, subresource apply, priority queue). https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.24.0 , https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.23.0
[^5]: Go module metadata for k8s.io/client-go v0.36.x, sigs.k8s.io/controller-runtime v0.24.x and sigs.k8s.io/controller-tools v0.21.0 (go directives, dependency versions, tag dates). https://pkg.go.dev/k8s.io/client-go@v0.36.5 , https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1 , https://pkg.go.dev/sigs.k8s.io/controller-tools@v0.21.0
[^6]: CHANGELOG-1.36, v1.36.0 "API Change", "Feature", "Deprecation" and "Urgent Upgrade Notes" sections. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md#v1360
[^7]: Kubernetes v1.37: Garhwal — release announcement (maxUnavailable for StatefulSets back on by default; kubernetes#137409). https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/
[^8]: Ingress NGINX Retirement: What You Need to Know; Announcing Ingress2Gateway 1.0. https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/ , https://kubernetes.io/blog/2026/03/20/ingress2gateway-1-0-release/
[^9]: CHANGELOG-1.36, patch releases v1.36.1–v1.36.5. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.36.md#v1365
[^10]: CHANGELOG-1.37 (v1.37.0 API changes, feature graduations, deprecations). https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.37.md
[^11]: Deprecated API Migration Guide. https://kubernetes.io/docs/reference/using-api/deprecation-guide/
[^12]: Volumes — `image` volume source. https://kubernetes.io/docs/concepts/storage/volumes/#image ; KEP-4639 OCI VolumeSource. https://github.com/kubernetes/enhancements/issues/4639
[^13]: User Namespaces (stable since v1.36; requirements, limitations, integration with Pod Security Admission). https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
[^14]: SELinux Volume Label Changes goes GA (and likely implications in v1.37). https://kubernetes.io/blog/2026/04/22/breaking-changes-in-selinux-volume-labeling/
[^15]: Kubelet authentication/authorization — fine-grained authorization (stable since v1.36). https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/
[^16]: Common Expression Language in Kubernetes — library availability table. https://kubernetes.io/docs/reference/using-api/cel/
[^17]: MutatingAdmissionPolicy. https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/ ; KEP-3962. https://github.com/kubernetes/enhancements/issues/3962
[^18]: ValidatingAdmissionPolicy. https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
[^19]: KEP-5707 Deprecate service.spec.externalIPs; KEP-5040 Remove gitRepo volume driver; KEP-4858 IP/CIDR validation; KEP-5440 Mutable Job pod resources. https://github.com/kubernetes/enhancements/issues/5707 , https://github.com/kubernetes/enhancements/issues/5040 , https://github.com/kubernetes/enhancements/issues/4858 , https://github.com/kubernetes/enhancements/issues/5440
[^20]: Pod Security Standards. https://kubernetes.io/docs/concepts/security/pod-security-standards/
[^21]: CustomResourceDefinition validation rules (transition rules, optionalOldSelf, messageExpression, reason, fieldPath, ratcheting, selectableFields). https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
