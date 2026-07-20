# Getting started

## Install

```sh
go install github.com/jjuanrivvera/linkedin-cli/cmd/linkedin@latest
```

## Authenticate (borrow your browser session)

```sh
linkedin auth --cookie-from-browser chrome
```

Or headless via env (the quotes in JSESSIONID are part of the value):

```sh
export LI_AT='AQED...'
export JSESSIONID='"ajax:1234567890"'
```

## First searches

```sh
linkedin jobs search --keywords golang --remote --since 7d -o json
linkedin geo "Bogota, Colombia"
linkedin jobs get 4012345678 --jq '.description.text'
linkedin company get stripe --jq '.name'
```
