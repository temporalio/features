import argparse
import asyncio
import importlib
import logging
from pathlib import Path
from typing import List, cast

from temporalio.service import TLSConfig

from harness.python.feature import Runner, features

logger = logging.getLogger(__name__)


async def run():
    # Parse args
    parser = argparse.ArgumentParser()
    parser.add_argument("--server", help="The host:port of the server", required=True)
    parser.add_argument("--namespace", help="The namespace to use", required=True)
    parser.add_argument(
        "--client-cert-path", help="Path to a client certificate for TLS"
    )
    parser.add_argument("--client-key-path", help="Path to a client key for TLS")
    parser.add_argument("--log-level", help="Log level", default="INFO")
    parser.add_argument("--http-proxy-url", help="HTTP proxy URL")
    parser.add_argument(
        "features", help="Features as dir + ':' + task queue", nargs="+"
    )
    args = parser.parse_args()

    tls_config = None
    if args.client_cert_path:
        if not args.client_key_path:
            raise ValueError("Client cert specified, but not client key!")

        with open(args.client_cert_path, "rb") as f:
            client_cert = f.read()
        with open(args.client_key_path, "rb") as f:
            client_key = f.read()
        tls_config = TLSConfig(client_cert=client_cert, client_private_key=client_key)
    elif args.client_key_path and not args.client_cert_path:
        raise ValueError("Client key specified, but not client cert!")

    # Configure logging
    logging.basicConfig(level=getattr(logging, args.log_level.upper()))

    # Collect all feature paths
    root_dir = Path(__file__, "../../../features").resolve()
    rel_dirs = sorted(
        v.relative_to(root_dir).parent.as_posix()
        for v in root_dir.glob("**/feature.py")
    )

    # Run each feature
    failure_count = 0
    for rel_dir_and_task_queue in cast(List[str], args.features):
        # Split rel dir and task queue
        rel_dir, _, task_queue = rel_dir_and_task_queue.partition(":")
        if rel_dir not in rel_dirs:
            raise ValueError(f"Cannot find feature file in {rel_dir}")
        # Import
        module = "features." + ".".join(rel_dir.split("/") + ["feature"])
        importlib.import_module(module)
        if rel_dir not in features:
            raise ValueError(f"Cannot find registered feature for {rel_dir}")
        # Run
        try:
            await Runner(
                address=args.server,
                namespace=args.namespace,
                task_queue=task_queue,
                feature=features[rel_dir],
                tls_config=tls_config,
                http_proxy_url=args.http_proxy_url if args.http_proxy_url else None,
            ).run()
        except Exception:
            logger.exception("Feature %s failed", rel_dir)
            failure_count += 1

    if failure_count:
        raise RuntimeError(f"{failure_count} feature(s) failed")
    logger.info("All features passed")


if __name__ == "__main__":
    asyncio.run(run())
