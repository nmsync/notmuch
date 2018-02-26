package notmuch

import (
	"io/ioutil"
	"os"
	"testing"
)

const message = `From: Sample Message <return@example.com>
Delivered-To: test@example.com
Return-Path: <return@example.com>
Content-Type: text/plain; charset=utf-8
Subject: Some test message
Date: Mon, 26 Feb 2018 00:00:00 +0200
Message-Id: <00000000-0000-0000-0000-000000000000@example.com>
To: Test Account <test@example.com>

This is some very sample message.

-- 
Regards.
`

func TestNotmuch(t *testing.T) {
	name, err := ioutil.TempDir("", "nm-")
	if err != nil {
		t.Fatalf("Could not create temp dir: %s", err)
	}
	defer os.RemoveAll(name)

	f, err := ioutil.TempFile("", "nm-")
	if err != nil {
		t.Fatalf("Could not create temp message file: %s", err)
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.WriteString(message); err != nil {
		t.Fatalf("Could not write message: %s", err)
	}
	f.Close()

	db, err := New(name)
	if err != nil {
		t.Fatalf("Could not create new notmuch DB: %s", err)
	}
	db.Close()

	db, err = Open(name, false)
	if err != nil {
		t.Fatalf("Could not open notmuch DB: %s", err)
	}
	defer db.Close()

	msg, err := db.FindMessage("doesnt-exist")
	if err != nil {
		t.Fatalf("Error in db.FindMessage: %s", err)
	}
	if msg != nil {
		t.Fatal("Message found where it should not exist")
	}

	msg, err = db.IndexFile(path)
	if err != nil {
		t.Fatalf("Error in IndexFile: %s", err)
	}
	id := msg.ID()
	t.Logf("Message %s indexed", id)

	msg, err = db.FindMessage(id)
	if err != nil {
		t.Fatalf("Error in db.FindMessage: %s", err)
	}
	if msg == nil {
		t.Fatalf("Message %s not found!", id)
	}

	tags := msg.Tags()
	t.Logf("Message tags: %v", tags)
	if len(tags) != 0 {
		t.Error("Tags() should be empty!")
	}

	if err = msg.AddTag("tag1"); err != nil {
		t.Errorf("Error in AddTag: %s", err)
	}
	tags = msg.Tags()
	t.Logf("Message tags: %v", tags)
	if len(tags) != 1 || tags[0] != "tag1" {
		t.Errorf("Invalid message tags: %v", tags)
	}

	if err = msg.AddTag("tag2"); err != nil {
		t.Errorf("Error in AddTag: %s", err)
	}
	tags = msg.Tags()
	t.Logf("Message tags: %v", tags)
	if len(tags) != 2 || tags[0] != "tag1" || tags[1] != "tag2" {
		t.Errorf("Invalid message tags: %v", tags)
	}

	if err = msg.Freeze(); err != nil {
		t.Fatalf("Error in Freeze: %s", err)
	}
	if err = msg.RemoveTag("tag1"); err != nil {
		t.Errorf("Error in RemoveTag: %s", err)
	}
	if err = msg.Thaw(); err != nil {
		t.Fatalf("Error in Thaw: %s", err)
	}
	tags = msg.Tags()
	t.Logf("Message tags: %v", tags)
	if len(tags) != 1 || tags[0] != "tag2" {
		t.Errorf("Invalid message tags: %v", tags)
	}

	if err = msg.RemoveAllTags(); err != nil {
		t.Errorf("Error in RemoveAllTags: %s", err)
	}
	tags = msg.Tags()
	t.Logf("Message tags: %v", tags)
	if len(tags) != 0 {
		t.Errorf("Invalid message tags: %v", tags)
	}
}
