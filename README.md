# Pijector

Pijector is yet another piece of Kiosk software designed for Raspberry Pi. It
uses the Chrome Devtools Protocol (via the [rod](https://github.com/go-rod/rod)
library) to control a Chromium instance as a Kiosk display.

## Usage

You will need to launch a Chromium instance in debug mode as a prerequisite. For
example:

```console
$ chromium-browser \
    --incognito \
    --kiosk \
    --no-first-run \
    --remote-debugging-port=9222 \
    --user-data-dir=$(mktemp -d) \
    -- \
        "about:blank"

DevTools listening on ws://127.0.0.1:9222/devtools/browser/b2d22b3a-dd60-4aa8-86fc-fa494d90e1dc
```

For testing and development purposes, you may use
[`bin/start-screen.sh`](bin/start-screen.sh).

Then, configure the Pijector server to control the chromium(s) you started. This
will require a bit of YAML.

```yaml
---
listen: 0.0.0.0:9292
default_url: https://en.wikipedia.org/wiki/Special:Random
screens:
  - name: Test Screen 1
    address: localhost:9223
```

Then run the server, given the above config as `pijector-dev.yml`:

```console
$ pijector server --help
NAME:
   pijector server - Run the pijector server

USAGE:
   pijector server [command options] [arguments...]

OPTIONS:
   --config value, -c value  Path to server configuration file.
   --help, -h                show help (default: false)
$ pijector server -c pijector-dev.yml
DEBU[0000] Loaded config: {Listen:0.0.0.0:9292 DefaultURL:https://en.wikipedia.org/wiki/Special:Random Screens:[{Name:Test Screen 1 Address:localhost:9223}]}
INFO[0000] attached to screen                            address="localhost:9223"
INFO[0000] server listening on 0.0.0.0:9292
```

As soon as the Pijector server starts, it will attempt to take control of the
Chromium instance, navigate it to the pijector default page, and bring it to the
foreground.

## Controlling the Pijector

Pijector exposes a simple admin user interface at `/admin` on its bound address
([localhost:9292/admin](http://localhost:9292/admin) by default).

![Pijector Admin Page Screenshot](doc/adminscreenshot.png)

## API

The Pijector API is quite simple. The admin interface uses it to discover and control screens, and other Pijectors may use it to control screens on remote Pijectors (see below).

- `GET /api/v1/screen` will return an object describing the screens controlled
  by the Pijector instance.

  For example:

  ```json
  {
    "screens": [
      {
        "url": "/api/v1/screen/91d21a4b-452d-43f7-a6bd-53797114242d",
        "id": "91d21a4b-452d-43f7-a6bd-53797114242d",
        "name": "Left Shoulder",
        "snap": "/api/v1/screen/91d21a4b-452d-43f7-a6bd-53797114242d/snap?2021-05-24T17:31:46Z",
        "display": {
          "title": "Aidan Roark - Wikipedia",
          "url": "https://en.wikipedia.org/wiki/Aidan_Roark"
        }
      },
      {
        "url": "/api/v1/screen/3b941997-b50f-4798-83ba-675c697dad61",
        "id": "3b941997-b50f-4798-83ba-675c697dad61",
        "name": "Remote Screen",
        "snap": "/api/v1/screen/3b941997-b50f-4798-83ba-675c697dad61/snap?2021-05-24T17:31:46Z",
        "display": {
          "title": "1945 All-Big Ten Conference football team - Wikipedia",
          "url": "https://en.wikipedia.org/wiki/1945_All-Big_Ten_Conference_football_team"
        }
      }
    ]
  }
  ```

- `GET /api/v1/screen/$SCREENID` will return details about the screen's current
  display.

  For example:

  ```json
  {
    "url": "/api/v1/screen/91d21a4b-452d-43f7-a6bd-53797114242d",
    "id": "91d21a4b-452d-43f7-a6bd-53797114242d",
    "name": "Left Shoulder",
    "snap": "/api/v1/screen/91d21a4b-452d-43f7-a6bd-53797114242d/snap?2021-05-24T17:31:46Z",
    "display": {
      "title": "Aidan Roark - Wikipedia",
      "url": "https://en.wikipedia.org/wiki/Aidan_Roark"
    }
  }
  ```

- `GET /api/v1/screen/$SCREENID/stat` is an alias for `/api/v1/screen/$SCREENID`

- `GET /api/v1/screen/$SCREENID/show?target=$TARGETURL` will instruct the screen
  to display the provided `$TARGETURL`. On success, it will wait for the page to
  be displayed, and then return a `/stat` payload (as above) with the new
  details.

- `GET /api/v1/screen/$SCREENID/snap` will return a full-resolution PNG
  screenshot of the screen's current display.


## Aggregating Screens

Since the Chromium / Google Chrome debugger refuses to bind to non-local
(`127.0.0.1`, etc) addresses, a Pijector server will not be able to directly
control a Chromium screen on another host. However, Pijector can use the API on
another Pijector instance to virtually attach screens from other Pijector
instances. This means that at least one Pijector server must be running on each
Pijector host, but they can all be aggregated for control on a single server.

Pijector will know what you mean if you configure the address of a screen to
contain a screen API URL. For example:

```yaml
---
listen: 0.0.0.0:9292
default_url: https://en.wikipedia.org/wiki/Special:Random
screens:
  - name: Local Screen
    address: localhost:9223
  - name: Remote Screen
    address: http://other.host:9292/api/v1/screen/3b941997-b50f-4798-83ba-675c697dad61
```
