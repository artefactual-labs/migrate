#!/usr/bin/env python3
"""Command-line interface used by migrate to simulate storage service commands."""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request


def main() -> int:
    parser = argparse.ArgumentParser(prog="ssmock-manage")
    subparsers = parser.add_subparsers(dest="command", required=True)

    replicate = subparsers.add_parser("create_aip_replicas")
    replicate.add_argument("--aip-uuid", required=True)
    replicate.add_argument("--aip-store-location", required=True)
    replicate.add_argument("--replicator-location", required=True)

    args = parser.parse_args()

    base_url = os.environ.get("SSMOCK_URL")
    if not base_url:
        print("SSMOCK_URL environment variable must be set", file=sys.stderr)
        return 1
    if base_url.endswith("/"):
        base_url = base_url[:-1]

    payload = json.dumps(
        {
            "aip_uuid": args.aip_uuid,
            "source_location_uuid": args.aip_store_location,
            "replica_location_uuid": args.replicator_location,
        }
    ).encode("utf-8")

    request = urllib.request.Request(
        url=f"{base_url}/_internal/replicate",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(request) as response:
            data = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        message = exc.read().decode("utf-8") or exc.reason
        print(f"CommandError: {message}", file=sys.stderr)
        return 1
    except Exception as exc:  # pylint: disable=broad-except
        print(f"CommandError: {exc}", file=sys.stderr)
        return 1

    status = data.get("status")
    if status == "success":
        print(f"INFO: Starting replication for {args.aip_uuid}")
        print(
            "New replicas created for 1 of 1 AIPs in location "
            f"{args.replicator_location}"
        )
        return 0
    if status == "noop":
        print(f"INFO: Replication already exists for {args.aip_uuid}")
        print(
            "New replicas created for 0 of 1 AIPs in location "
            f"{args.replicator_location}."
        )
        return 0
    if status == "missing":
        print(
            "CommandError: No AIPs to replicate in location "
            f"{args.aip_store_location}"
        )
        return 1

    print(f"CommandError: {data.get('message', 'replication failed')}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
