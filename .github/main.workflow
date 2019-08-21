workflow "Build" {
  on = "push"
  resolves = ["Static Analysis: Copyright", "Static Analysis: Shellcheck"]
}

action "Static Analysis: Copyright" {
  uses = "./.github/static_analysis/"
  args = "--build-arg ARGS=test_copyright"
}

action "Static Analysis: Shellcheck" {
  uses = "./.github/static_analysis/"
  args = "--build-arg ARGS=test_static_analysis_shell"
}
