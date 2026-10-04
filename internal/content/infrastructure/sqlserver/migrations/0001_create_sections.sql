CREATE TABLE dbo.sections (
    id          NCHAR(36)      NOT NULL CONSTRAINT pk_sections PRIMARY KEY,
    kind        NVARCHAR(32)   NOT NULL CONSTRAINT uq_sections_kind UNIQUE,
    draft_title NVARCHAR(200)  NOT NULL,
    draft_body  NVARCHAR(MAX)  NOT NULL,
    created_at  DATETIMEOFFSET NOT NULL,
    updated_at  DATETIMEOFFSET NOT NULL,
    revision    BIGINT         NOT NULL
);
GO

CREATE TABLE dbo.section_versions (
    section_id   NCHAR(36)      NOT NULL CONSTRAINT fk_section_versions_section REFERENCES dbo.sections (id),
    number       INT            NOT NULL,
    title        NVARCHAR(200)  NOT NULL,
    body         NVARCHAR(MAX)  NOT NULL,
    published_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT pk_section_versions PRIMARY KEY (section_id, number)
);
GO
