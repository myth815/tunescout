#!/usr/bin/env python3
"""Small dependency-free TuneScout client for agent workflows."""

from __future__ import annotations

import argparse
import json
import mimetypes
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path


def settings() -> tuple[str, dict[str, str]]:
    base = os.environ.get("TUNESCOUT_BASE_URL", "").rstrip("/")
    if not base:
        raise SystemExit("TUNESCOUT_BASE_URL is required")
    headers = {"Accept": "application/json", "User-Agent": "TuneScout-OpenClaw-Skill/0.1"}
    api_key = os.environ.get("TUNESCOUT_API_KEY", "").strip()
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    return base, headers


def request_json(url: str, headers: dict[str, str], body: bytes | None = None) -> object:
    request = urllib.request.Request(url, data=body, headers=headers, method="POST" if body is not None else "GET")
    try:
        with urllib.request.urlopen(request, timeout=50) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read(8192).decode("utf-8", "replace")
        raise SystemExit(f"TuneScout returned HTTP {error.code}: {detail}") from error
    except urllib.error.URLError as error:
        raise SystemExit(f"TuneScout request failed: {error.reason}") from error


def split_csv(value: str) -> list[str]:
    return [item.strip() for item in value.split(",") if item.strip()]


def parse_filters(values: list[str]) -> dict[str, object]:
    filters: dict[str, object] = {}
    for value in values:
        if "=" not in value:
            raise SystemExit(f"invalid filter {value!r}; expected KEY=VALUE")
        key, raw = value.split("=", 1)
        if key in {"duration_min_ms", "duration_max_ms"}:
            try:
                filters[key] = int(raw)
            except ValueError as error:
                raise SystemExit(f"filter {key} must be an integer") from error
        else:
            filters[key] = raw
    return filters


def search_payload(args: argparse.Namespace) -> dict[str, object]:
    payload: dict[str, object] = {"limit": args.limit}
    if args.query:
        payload["query"] = args.query
    if args.types:
        payload["types"] = split_csv(args.types)
    if args.region or args.providers:
        strategy: dict[str, object] = {}
        if args.region:
            strategy["region"] = args.region
        if args.providers:
            strategy["providers"] = split_csv(args.providers)
        payload["strategy"] = strategy
    if args.filter:
        payload["filters"] = parse_filters(args.filter)
    if args.input:
        try:
            payload["inputs"] = [json.loads(item) for item in args.input]
        except json.JSONDecodeError as error:
            raise SystemExit(f"invalid --input JSON: {error}") from error
    return payload


def multipart(payload: dict[str, object], audio_path: Path) -> tuple[bytes, str]:
    if not audio_path.is_file():
        raise SystemExit(f"audio file not found: {audio_path}")
    boundary = "tunescout-" + uuid.uuid4().hex
    content_type = mimetypes.guess_type(audio_path.name)[0] or "application/octet-stream"
    chunks = [
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"request\"\r\nContent-Type: application/json\r\n\r\n".encode(),
        json.dumps(payload, ensure_ascii=False).encode(),
        f"\r\n--{boundary}\r\nContent-Disposition: form-data; name=\"audio\"; filename=\"{audio_path.name}\"\r\nContent-Type: {content_type}\r\n\r\n".encode(),
        audio_path.read_bytes(),
        f"\r\n--{boundary}--\r\n".encode(),
    ]
    return b"".join(chunks), f"multipart/form-data; boundary={boundary}"


def run_search(args: argparse.Namespace) -> object:
    base, headers = settings()
    payload = search_payload(args)
    if args.audio:
        body, content_type = multipart(payload, Path(args.audio))
        headers["Content-Type"] = content_type
    else:
        body = json.dumps(payload, ensure_ascii=False).encode()
        headers["Content-Type"] = "application/json"
    return request_json(base + "/v1/search", headers, body)


def run_entity(args: argparse.Namespace) -> object:
    base, headers = settings()
    query = urllib.parse.urlencode({"include": args.include}) if args.include else ""
    url = base + "/v1/entities/" + urllib.parse.quote(args.entity_ref, safe="")
    return request_json(url + ("?" + query if query else ""), headers)


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    search = commands.add_parser("search", help="search with text, structured clues, or audio")
    search.add_argument("query", nargs="?", default="")
    search.add_argument("--types", default="artist,recording,release")
    search.add_argument("--limit", type=int, default=20)
    search.add_argument("--region", default="")
    search.add_argument("--providers", default="")
    search.add_argument("--filter", action="append", default=[])
    search.add_argument("--input", action="append", default=[], help="JSON Input object; repeatable")
    search.add_argument("--audio", default="", help="short audio excerpt")
    search.set_defaults(handler=run_search)
    entity = commands.add_parser("entity", help="expand an entity_ref")
    entity.add_argument("entity_ref")
    entity.add_argument("--include", default="")
    entity.set_defaults(handler=run_entity)
    return root


def main() -> None:
    args = parser().parse_args()
    result = args.handler(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
