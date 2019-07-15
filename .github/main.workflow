workflow "Build" {
  on = "push"
  resolves = "Lint"
}

action "Lint" {
  uses = "./.github/lint/"
  runs = "make"
  args = "lint"
}
