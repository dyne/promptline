//go:build windows

package skills

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileRenameInformation struct {
	replaceIfExists uint32
	rootDirectory   windows.Handle
	fileNameLength  uint32
	fileName        [1]uint16
}

func renameNoReplace(oldRoot *os.Root, oldName string, newRoot *os.Root, newName string) error {
	oldDirectory, err := oldRoot.Open(".")
	if err != nil {
		return err
	}
	defer oldDirectory.Close()
	newDirectory, err := newRoot.Open(".")
	if err != nil {
		return err
	}
	defer newDirectory.Close()

	objectName, err := windows.NewNTUnicodeString(oldName)
	if err != nil {
		return err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(oldDirectory.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	var status windows.IO_STATUS_BLOCK
	var source windows.Handle
	err = windows.NtCreateFile(
		&source,
		windows.DELETE|windows.SYNCHRONIZE,
		attributes,
		&status,
		nil,
		0,
		windows.FILE_SHARE_DELETE|windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_FOR_BACKUP_INTENT|
			windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(source)

	newNameUTF16, err := windows.UTF16FromString(newName)
	if err != nil {
		return err
	}
	fileNameBytes := (len(newNameUTF16) - 1) * 2
	if fileNameBytes/2 > syscall.MAX_PATH {
		return windows.ERROR_FILENAME_EXCED_RANGE
	}
	var layout fileRenameInformation
	bufferSize := int(unsafe.Offsetof(layout.fileName)) + fileNameBytes
	buffer := make([]byte, bufferSize)
	information := (*fileRenameInformation)(unsafe.Pointer(&buffer[0]))
	information.rootDirectory = windows.Handle(newDirectory.Fd())
	information.fileNameLength = uint32(fileNameBytes)
	copy(
		(*[syscall.MAX_PATH]uint16)(unsafe.Pointer(&information.fileName[0]))[:fileNameBytes/2:fileNameBytes/2],
		newNameUTF16,
	)
	return windows.NtSetInformationFile(
		source,
		&status,
		&buffer[0],
		uint32(bufferSize),
		windows.FileRenameInformation,
	)
}
