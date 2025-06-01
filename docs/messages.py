# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "babel",
# ]
# ///

# SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

import os
import shutil
from argparse import ArgumentParser
from operator import itemgetter
from pathlib import Path

from babel.messages.catalog import Catalog
from babel.messages.extract import extract_from_file
from babel.messages.pofile import read_po, write_po

HERE = Path(__file__).parent
ROOT = HERE / "src"

CATALOG_HEADER = """\
# Translations template for PROJECT.
# SPDX-FileCopyrightText: © YEAR Readeck <translate@readeck.com>
#
# SPDX-License-Identifier: AGPL-3.0-only
#"""

CATALOG_OPTIONS = {
    "header_comment": CATALOG_HEADER,
    "project": "Readeck User Documentation",
    "version": "1.0.0",
    "copyright_holder": "Readeck",
    "msgid_bugs_address": "translate@readeck.com",
    "last_translator": "Readeck <translate@readeck.com>",
    "language_team": "Readeck <translate@readeck.com>",
}


def extract_blocks(fileobj, keywords, comment_tags, options):
    token = None
    messages = []
    for lineno, text in enumerate(fileobj):
        lineno = lineno + 1

        if token is None:
            token = [lineno, "", [], []]
            messages = []

        if text.strip() != b"":
            messages.append(text.decode("utf-8").rstrip())
        else:
            if len(messages) > 0:
                token[2] = "\n".join(messages)
                yield token
            token = None
            messages = []

    if token is not None and len(messages) > 0:
        token[2] = "\n".join(messages)
        yield token


def po2text(catalog: Catalog, destdir: Path):
    os.makedirs(destdir, exist_ok=True)
    files = {}

    for m in catalog._messages.values():
        for x in m.locations:
            name = Path(x[0]).name
            files.setdefault(name, [])

            msg = m.string
            if m.fuzzy or msg.strip() == "":
                msg = m.id
            files[name].append((x[1], msg))

    for k in files:
        files[k] = sorted(files[k], key=itemgetter(0))

    for k, messages in files.items():
        dest = destdir / k
        with dest.open("w") as fp:
            for x in messages:
                fp.write(x[1])
                fp.write("\n\n")
            yield dest


def extract(_):
    template = Catalog(**CATALOG_OPTIONS)

    for f in (ROOT / "en-US").rglob("*.md"):
        for lineno, message, comments, context in extract_from_file(
            extract_blocks,
            f,
        ):
            template.add(
                message,
                None,
                [(str(f.relative_to(ROOT)), lineno)],
                auto_comments=comments,
                context=context,
            )

    translations = HERE / "translations"
    dest = translations / "messages.pot"
    with dest.open("wb") as fp:
        write_po(
            fp,
            template,
            width=None,
            sort_by_file=True,
            include_lineno=True,
            ignore_obsolete=True,
        )
        print(f"{dest} writen")


def update(_):
    translations = HERE / "translations"
    with (translations / "messages.pot").open("rb") as fp:
        template = read_po(fp)

    dirs = [x for x in translations.iterdir() if x.is_dir()]
    for p in dirs:
        po_file = p / "messages.po"
        if po_file.exists():
            with po_file.open("rb") as fp:
                catalog = read_po(fp, locale=p.name, domain=po_file.name)
        else:
            catalog = Catalog(
                **CATALOG_OPTIONS,
                locale=p.name,
                domain=po_file.name,
            )

        catalog.update(template)

        with po_file.open("wb") as fp:
            write_po(
                fp,
                catalog,
                width=None,
                sort_by_file=True,
                include_lineno=True,
                include_previous=True,
            )
            print(f"{po_file} written")


def generate(_):
    translations = HERE / "translations"
    po_files = translations.glob("*/messages.po")

    for po_file in po_files:
        code = po_file.parent.name

        if code == "en_US":
            continue

        # Write markdown files
        with po_file.open("rb") as fp:
            catalog = read_po(fp)
            destdir = HERE / "src" / str(catalog.locale_identifier).replace("_", "-")
            os.makedirs(destdir, exist_ok=True)
            print(f"{po_file} -> {destdir}")
            for x in po2text(catalog, destdir):
                print(f"  - {x} written")

        # Copy missing images
        os.makedirs(destdir / "img", exist_ok=True)
        for x in ((destdir.parent) / "en-US/img").iterdir():
            if not x.is_file:
                continue

            if not (destdir / "img" / x.name).exists():
                shutil.copy2(x, destdir / "img" / x.name)
                print(f"{x} -> {destdir / 'img' / x.name}")


def main():
    parser = ArgumentParser()
    subparsers = parser.add_subparsers(required=True)

    p_extract = subparsers.add_parser("extract", help="Extract messages")
    p_extract.set_defaults(func=extract)

    p_update = subparsers.add_parser("update", help="Update strings")
    p_update.set_defaults(func=update)

    p_generate = subparsers.add_parser("generate", help="generate markdown files")
    p_generate.set_defaults(func=generate)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
