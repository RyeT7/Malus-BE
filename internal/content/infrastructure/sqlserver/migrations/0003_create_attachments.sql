CREATE TABLE dbo.attachments (
    id           NCHAR(36)      NOT NULL CONSTRAINT pk_attachments PRIMARY KEY,
    file_name    NVARCHAR(200)  NOT NULL,
    content_type NVARCHAR(100)  NOT NULL,
    size_bytes   BIGINT         NOT NULL,
    status       NVARCHAR(16)   NOT NULL CONSTRAINT ck_attachments_status CHECK (status IN (N'pending', N'ready')),
    created_at   DATETIMEOFFSET NOT NULL,
    ready_at     DATETIMEOFFSET NULL
);
GO
