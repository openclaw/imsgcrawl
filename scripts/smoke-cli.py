"""Exercise the built CLI using only an owned synthetic Messages database."""

import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile


SCHEMA = """
create table handle (
  rowid integer primary key, id text not null, service text not null,
  uncanonicalized_id text
);
create table chat (
  rowid integer primary key, guid text not null, display_name text,
  chat_identifier text, service_name text, room_name text, is_archived integer
);
create table chat_handle_join (chat_id integer, handle_id integer);
create table message (
  rowid integer primary key, guid text not null, handle_id integer, date integer,
  service text, is_from_me integer, text text, attributedBody blob,
  date_edited integer default 0, date_retracted integer default 0,
  message_summary_info blob
);
create table chat_message_join (chat_id integer, message_id integer);
create table message_attachment_join (message_id integer, attachment_id integer);
create table sync_deleted_messages (guid text);
create table sync_deleted_chats (guid text);
insert into handle values
  (1, '+15550100', 'iMessage', ''), (2, '0015550100', 'SMS', ''),
  (3, 'fake@example.test', 'iMessage', '');
insert into chat values
  (1, 'chat-one', 'Synthetic Friend', '+15550100', 'iMessage', '', 0);
insert into chat_handle_join values (1, 1);
insert into message(rowid,guid,handle_id,date,service,is_from_me,text) values
  (1,'message-one',1,800000000000000000,'iMessage',0,'synthetic orchard --help --json'),
  (2,'message-two',2,800000001000000000,'SMS',1,'synthetic reply');
insert into chat_message_join values (1,1), (1,2);
"""


def smoke(binary, directory):
    source, archive = directory / "source.db", directory / "archive.db"
    with sqlite3.connect(source) as db:
        db.executescript(SCHEMA)
        db.execute("update message set date=500000000 where rowid=1")
    before = hashlib.sha256(source.read_bytes()).digest()

    def run(*args, machine=True, archive_path=archive):
        argv = [str(binary), "--db", str(source), "--archive", str(archive_path)]
        if machine:
            argv.append("--json")
        result = subprocess.run(
            argv + list(args), text=True, capture_output=True, check=True,
            timeout=30, env={**os.environ, "TZ": "UTC", "COLUMNS": "100"},
        )
        return json.loads(result.stdout) if machine else result.stdout

    assert run("metadata")["id"] == "imsgcrawl"
    assert run("sync")["messages"] == 2
    assert run("sync")["mode"] == "merge"
    assert run("status")["state"] == "ok"
    path_status = run("status", archive_path=str(archive) + " ")
    assert path_status["archive"]["archive_bytes"] == archive.stat().st_size
    assert path_status["archive"]["archive_path"] == str(archive) + " "
    assert run("chats")["items"][0]["message_count"] == 2
    assert run("messages", "--chat", "1")["returned"] == 2
    assert "2016-11-05 00:53" in run("messages", "--chat", "1", machine=False)
    assert run("messages", "--chat", "1", "--asc")["items"][0]["date"] == 500000000
    assert run("search", "orchard")["returned"] == 1
    for query in ("--help", "--json"):
        literal = run("search", "--", query)
        assert literal["query"] == query and literal["returned"] == 1
    assert "Usage:" in run("--help", machine=False)
    contacts = run("contacts", "export")["contacts"]
    assert len(contacts) == 1
    assert contacts[0]["phone_numbers"] == ["0015550100"]
    for args in (
        ("status",), ("chats",), ("messages", "--chat", "1"),
        ("search", "orchard"), ("contacts", "export"),
    ):
        assert run(*args, machine=False).strip()
    assert run("sync", "--restore")["mode"] == "restore"
    assert hashlib.sha256(source.read_bytes()).digest() == before
    print("PASS: synthetic CLI metadata, merge, status, chats, messages, search, "
          "contacts, text/JSON and restore; source bytes unchanged.")


if __name__ == "__main__":
    binary = Path(sys.argv[1]).resolve()
    with tempfile.TemporaryDirectory(prefix="imsgcrawl-proof-") as temporary:
        smoke(binary, Path(temporary))
