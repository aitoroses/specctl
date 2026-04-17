# Agent quickstart

## 1. Install the packaged skill

```bash
npx skills add https://github.com/aitoroses/specctl --skill specctl --global
```

## 2. Install/configure specctl

From an installed skill checkout or cloned repo:

```bash
bash skills/specctl/scripts/setup.sh
```

## 3. Start with context

```bash
specctl context specctl:cli
```

## 4. Use the governed workflow

Typical loop:

```bash
specctl delta add ...
specctl req add ...
specctl req verify ...
specctl delta close ...
specctl rev bump ...
```
