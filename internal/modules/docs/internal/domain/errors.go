package domain

import "errors"

var (
	ErrDocumentNotFound        = errors.New("document not found")
	ErrDocumentAlreadyExists   = errors.New("document already exists")
	ErrAttachmentNotFound      = errors.New("attachment not found")
	ErrAttachmentAlreadyExists = errors.New("attachment already exists for this reference")
	ErrRetentionPolicyNotFound = errors.New("retention policy not found")
	ErrStorageConfigNotFound   = errors.New("storage config not found")
	ErrInvalidFileSize         = errors.New("file size exceeds maximum allowed")
	ErrInvalidFileType         = errors.New("file type not allowed")
	ErrDocumentOnLegalHold     = errors.New("document is on legal hold and cannot be modified")
	ErrDocumentExpired         = errors.New("document has expired")
	ErrDocumentArchived        = errors.New("document is archived")
	ErrDocumentDeleted         = errors.New("document has been deleted")
	ErrInvalidDocumentStatus   = errors.New("invalid document status for this operation")
	ErrVersionNotFound         = errors.New("document version not found")
	ErrChecksumMismatch        = errors.New("file checksum does not match")
	ErrStorageError            = errors.New("storage operation failed")
	ErrInvalidStoragePath      = errors.New("invalid storage path")
)
