# Minimal governed workflow

```bash
specctl context specctl:cli
specctl delta add ...
# edit SPEC.md
specctl req add ...
# implement code/tests
specctl req verify ...
specctl delta close ...
specctl rev bump ...
```

Key rule:

- write meaning in `SPEC.md`
- use `specctl` to mutate tracking state
