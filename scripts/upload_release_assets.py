#!/usr/bin/env python3
"""Upload release assets to a GitCode release.

GitCode does not accept a single-request upload like GitHub. The asset has to
go through a two-step flow:

  1. GET  /api/v5/repos/{owner}/{repo}/releases/{tag}/upload_url?file_name=...
     returns a pre-signed object-storage URL plus the headers to send with it
  2. PUT  the file bytes to that URL with those headers

The asset names must match what `cf upgrade` builds, otherwise the upgrade
command cannot find the download URL:

  cf_<tag>_<goos>_<32|64>.zip   e.g. cf_v1.0.6_darwin_64.zip

Build them with:

  for os in darwin linux windows; do
    for arch in amd64 386; do
      bits=64; [ $arch = 386 ] && bits=32
      out=cf; [ $os = windows ] && out=cf.exe
      GOOS=$os GOARCH=$arch go build -o "$out" .
      zip -j "cf_${TAG}_${os}_${bits}.zip" "$out" && rm -f "$out"
    done
  done

Usage:
  export CF_TOKEN=<gitcode personal access token>
  ./upload_release_assets.py --tag v1.0.6 dist/*.zip

  # upload to another repository
  ./upload_release_assets.py --repo owner/name --tag v1.0.0 dist/*.zip
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

API = "https://api.gitcode.com/api/v5/repos"


def parse_args():
    parser = argparse.ArgumentParser(description="Upload assets to a GitCode release")
    parser.add_argument("--repo", default="sheng_wang/cf-tool_modify",
                        help="owner/name (default: sheng_wang/cf-tool_modify)")
    parser.add_argument("--tag", required=True, help="release tag, e.g. v1.0.6")
    parser.add_argument("files", nargs="+", help="asset files to upload")
    return parser.parse_args()


def api_get(url, token):
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def upload(url, headers, path):
    data = open(path, "rb").read()
    req = urllib.request.Request(url, data=data, method="PUT")
    for key, value in headers.items():
        req.add_header(key, value)
    with urllib.request.urlopen(req) as resp:
        return resp.status


def main():
    args = parse_args()
    token = os.environ.get("CF_TOKEN") or os.environ.get("GITCODE_TOKEN")
    if not token:
        sys.exit("Set CF_TOKEN (or GITCODE_TOKEN) to a GitCode personal access token")

    base = "%s/%s/releases/%s" % (API, args.repo, urllib.parse.quote(args.tag))
    failed = []
    for path in args.files:
        name = os.path.basename(path)
        try:
            info = api_get("%s/upload_url?file_name=%s" % (base, urllib.parse.quote(name)), token)
            status = upload(info["url"], info["headers"], path)
            print("%s: %s" % (name, status))
        except urllib.error.HTTPError as err:
            print("%s: failed %s %s" % (name, err.code, err.read()[:200]))
            failed.append(name)
        except OSError as err:
            print("%s: failed %s" % (name, err))
            failed.append(name)

    if failed:
        sys.exit("Failed to upload: %s" % ", ".join(failed))


if __name__ == "__main__":
    main()
