# Manifold Github Auto-Tagging Bot

Auto-tags releases.

```
NEVER_FAIL        never returns an error. Returns EX_CONFIG instead.
NO_EX_CONFIG      disables the special Github EX_CONFIG return, returning
                  success instead. This prevents parallel actions from being
                  interrupted                  
```

To use with Github Actions:

```yaml
on:
  pull_request:
    types: [ closed ]
  
name: autotag versions
jobs:
  autotag:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@master
    - name: autotag
      uses: manifoldco/autotagger@master
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        NEVER_FAIL: "true"
        NO_EX_CONFIG: "true"
```
