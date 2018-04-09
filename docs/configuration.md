---
title:        Configuration
layout:       default
class:        html
docson:       true
marked:       true
ejs:          true
superagent:   true
docref:       true
order:        1
---

`taskcluster-worker` takes a powerful configuration file on the form:

```yaml
transforms:
  - env
  - ... # Configuration transforms applied in order listed
config:
  concurrency: 4
  ... # Configuration keys that will be transformed.
```

Once configuration transforms have been applied to the `config` section the
final object must have a form that satisfies the schema in `config-schema.json'

## Transform `abs`

## Transform `env`

## Transform `hostcredentials`

## Transform `packet`

## Transform `secrets`
