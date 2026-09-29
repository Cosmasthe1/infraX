package infrax.policies

deny[msg] {
  input.kind == "Pod"
  container := input.spec.containers[_]
  container.securityContext.allowPrivilegeEscalation == true
  msg = sprintf("privileged containers forbidden: %s", [container.name])
}
