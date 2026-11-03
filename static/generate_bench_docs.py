#!/usr/bin/env python3
"""Generate JSON Lines benchmark documents for be_indexer."""

import argparse
import json
import random
import string
from pathlib import Path
from typing import Optional, TextIO

DEFAULT_COUNT = 100_000
DEFAULT_OUTPUT = Path(__file__).resolve().parent / "testdata" / "bench_docs.json"
CHARACTERS = string.ascii_lowercase + string.digits


def random_token(rng: random.Random, length: int = 8) -> str:
    return "".join(rng.choices(CHARACTERS, k=length))


def make_document(doc_id: int, rng: random.Random) -> dict:
    """Build one document with binary, numeric, categorical, and age dimensions."""
    document = {
        "id": doc_id,
        # Two-value numeric dimension, e.g. a binary gender/category encoding.
        "gender": rng.randrange(2),
    }

    # Three independently randomized numeric dimensions.
    for i in range(1, 4):
        document[f"number_{i}"] = rng.randint(0, 1_000_000)

    # Five random string dimensions.
    for i in range(1, 6):
        document[f"char_{i}"] = random_token(rng)

    # Five age-like numeric dimensions, suitable for range predicates.
    for i in range(1, 6):
        document[f"age_{i}"] = rng.randint(0, 120)

    return document


def write_documents(output: Path, count: int, seed: Optional[int] = None) -> None:
    rng = random.Random(seed)
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", encoding="utf-8", newline="\n") as file:  # type: TextIO
        for doc_id in range(count):
            file.write(json.dumps(make_document(doc_id, rng), ensure_ascii=False))
            file.write("\n")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--count", type=int, default=DEFAULT_COUNT,
        help=f"number of documents to generate (default: {DEFAULT_COUNT})",
    )
    parser.add_argument(
        "--seed", type=int, default=None,
        help="optional random seed for reproducible output",
    )
    parser.add_argument(
        "--output", type=Path, default=DEFAULT_OUTPUT,
        help=f"output JSON Lines file (default: {DEFAULT_OUTPUT})",
    )
    args = parser.parse_args()

    if args.count < 0:
        parser.error("--count must be non-negative")

    write_documents(args.output, args.count, args.seed)
    print(f"Wrote {args.count:,} JSON lines to {args.output}")


if __name__ == "__main__":
    main()
