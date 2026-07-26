## pal-pal is a simple web interface for the Palworld Dedicated Server REST API.

Features:
- View current activity on the server
- View player list (kick/ban/unban for admins)
- Map view (with player locations)
- View metrics history

Tech stack:
- Go
- Native ServeMux routing
- html/template
- Htmx
- Sqlite (modernc.org/sqlite for simplicity; no cgo)
- Cookie session auth (in-mem; reset on restart)

Parts:
- Monitor: Calls API, maintains live snapshot, records to sqlite at regular intervals (per-series)
- Web: Serves the web interface; split into view routes and component routes (htmx); minimal to no JS (htmx handles most of it)

API reference: https://docs.palworldgame.com/category/rest-api/