# CircleCI library and simple CLI

A partially finished API client and a simple CLI to fetch the state of a CircleCI pipeline on a particular branch.

```
$ ./circleci
  Merge pull request #77028 from foo/bar
✓ Merge pull request #77051 from foo/baz
✗ Merge pull request #76661 from foo/bad
    run_foo_tests failed <link to failing job>
```

You can examine one pipeline by specifying a substring, which is used to match against the subject:

```
$ ./circleci foo/bar
  Merge pull request #77028 from foo/bar
    run_foo_tests running
    run_bar_tests blocked
```

Configuration is via environment variables. .env file supported:

```
CIRCLECI_TOKEN=...
CIRCLECI_ORG_SLUG=...
CIRCLECI_ORG=...
CIRCLECI_PROJECT=...
CIRCLECI_BRANCH=...
```

Note that the org slug is in the form `<vcs provider>/<org name>`, e.g. `gh/dooferlad`.

I haven't gone through working out how to reference other VCS providers, so at the moment pipeline links in the
CLI only reference GitHub. I am sure the right way to do this is hidden in the CircleCI API somewhere - PRs welcome.