// A thin wrapper around notmuch C library. Only as much functionality as
// needed by nmsync daemon.
package notmuch // import "github.com/nmsync/notmuch"

/*
#cgo LDFLAGS: -lnotmuch

#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "notmuch.h"
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

type status C.notmuch_status_t

const (
	statusSuccess            status = C.NOTMUCH_STATUS_SUCCESS
	statusDuplicateMessageID status = C.NOTMUCH_STATUS_DUPLICATE_MESSAGE_ID
)

func (s status) Error() string {
	return fmt.Sprintf("notmuch: %s", C.GoString(C.notmuch_status_to_string(C.notmuch_status_t(s))))
}

func statusToError(st status) error {
	if st == statusSuccess || st == statusDuplicateMessageID {
		return nil
	} else {
		return st
	}
}

type Database struct {
	db *C.notmuch_database_t
}

// Create a new, empty notmuch database located at 'path'.
//
// The path should be a top-level directory to a collection of plain-text email
// messages (one message per file). This call will create a new ".notmuch"
// directory within 'path' where notmuch will store its data.
func New(path string) (*Database, error) {
	var db Database
	cPath := C.CString(path)
	st := status(C.notmuch_database_create(cPath, &db.db))
	C.free(unsafe.Pointer(cPath))
	if st != statusSuccess {
		return nil, st
	}
	return &db, nil
}

// Open an existing notmuch database located at 'path'.
//
// The database should have been created at some time in the past, (not
// necessarily by this process), by calling New with 'path'.
func Open(path string, readOnly bool) (*Database, error) {
	var db Database
	var mode C.notmuch_database_mode_t
	if readOnly {
		mode = C.NOTMUCH_DATABASE_MODE_READ_ONLY
	} else {
		mode = C.NOTMUCH_DATABASE_MODE_READ_WRITE
	}
	cPath := C.CString(path)
	st := status(C.notmuch_database_open(cPath, mode, &db.db))
	C.free(unsafe.Pointer(cPath))
	if st != statusSuccess {
		return nil, st
	}
	return &db, nil
}

// Close the given notmuch database, freeing all associated resources.
func (db *Database) Close() error {
	return statusToError(status(C.notmuch_database_destroy(db.db)))
}

// Does this database need to be upgraded before writing to it?
func (db *Database) NeedsUpgrade() bool {
	needsUpgrade := C.notmuch_database_needs_upgrade(db.db)
	return needsUpgrade != 0
}

// Add a message file to a database, indexing it for retrieval by future
// searches.  If a message already exists with the same message ID as the
// specified file, their indexes will be merged, and this new filename will
// also be associated with the existing message.
func (db *Database) IndexFile(path string) (*Message, error) {
	var msg Message
	cPath := C.CString(path)
	st := status(C.notmuch_database_index_file(db.db, cPath, nil, &msg.msg))
	C.free(unsafe.Pointer(cPath))
	switch st {
	case statusSuccess, statusDuplicateMessageID:
		runtime.SetFinalizer(&msg, finalizeMessage)
		return &msg, nil
	default:
		return nil, st
	}
}

// Remove a message filename from the given notmuch database. If the message
// has no more filenames, remove the message.
//
// If the same message (as determined by the message ID) is still available via
// other filenames, then the message will persist in the database for those
// filenames. When the last filename is removed for a particular message, the
// database content for that message will be entirely removed.
func (db *Database) RemoveMessage(path string) (hasMore bool, err error) {
	cPath := C.CString(path)
	st := status(C.notmuch_database_remove_message(db.db, cPath))
	C.free(unsafe.Pointer(cPath))
	switch st {
	case statusSuccess:
		return false, nil
	case statusDuplicateMessageID:
		return true, nil
	default:
		return false, st
	}
}

// Find a message with the given id.
//
// Returns nil if message with the given id is not found.
func (db *Database) FindMessage(id string) (*Message, error) {
	var msg Message
	cID := C.CString(id)
	st := status(C.notmuch_database_find_message(db.db, cID, &msg.msg))
	C.free(unsafe.Pointer(cID))
	if st != statusSuccess {
		return nil, st
	}
	if msg.msg == nil {
		return nil, nil
	}
	runtime.SetFinalizer(&msg, finalizeMessage)
	return &msg, nil
}

type Message struct {
	msg *C.notmuch_message_t
}

func finalizeMessage(msg *Message) {
	C.notmuch_message_destroy(msg.msg)
}

// Get the message ID.
func (m *Message) ID() string {
	id := C.notmuch_message_get_message_id(m.msg)
	return C.GoString(id)
}

// Get a filename for the message.
func (m *Message) FileName() string {
	path := C.notmuch_message_get_filename(m.msg)
	return C.GoString(path)
}

// Return a list of tags for the message.
func (m *Message) Tags() (tags []string) {
	cTags := C.notmuch_message_get_tags(m.msg)
	if cTags == nil {
		return
	}
	for v := C.notmuch_tags_valid(cTags); v != 0; v = C.notmuch_tags_valid(cTags) {
		s := C.notmuch_tags_get(cTags)
		tags = append(tags, C.GoString(s))
		C.notmuch_tags_move_to_next(cTags)
	}
	C.notmuch_tags_destroy(cTags)
	return
}

// Add a tag to the message.
func (m *Message) AddTag(tag string) error {
	cTag := C.CString(tag)
	defer C.free(unsafe.Pointer(cTag))
	return statusToError(status(C.notmuch_message_add_tag(m.msg, cTag)))
}

// Remove a tag from the message.
func (m *Message) RemoveTag(tag string) error {
	cTag := C.CString(tag)
	defer C.free(unsafe.Pointer(cTag))
	return statusToError(status(C.notmuch_message_remove_tag(m.msg, cTag)))
}

// Remove all tags from the message.
func (m *Message) RemoveAllTags() error {
	return statusToError(status(C.notmuch_message_remove_all_tags(m.msg)))
}

// Freeze the current state of the message within the database.
//
// This means that changes to the message state, (via Message.AddTag(),
// Message.RemoveTag(), and Message.RemoveAllTags()), will not be committed to
// the database until the message is thawed with Thaw().
func (m *Message) Freeze() error {
	return statusToError(status(C.notmuch_message_freeze(m.msg)))
}

// Thaw the message, synchronizing any changes that may have occurred while
// message was frozen into the notmuch database.
func (m *Message) Thaw() error {
	return statusToError(status(C.notmuch_message_thaw(m.msg)))
}
