ALTER TABLE dbo.sections
    ADD draft_items NVARCHAR(MAX) NOT NULL CONSTRAINT df_sections_draft_items DEFAULT N'[]';
GO

ALTER TABLE dbo.sections
    ADD CONSTRAINT ck_sections_draft_items_json CHECK (ISJSON(draft_items) = 1);
GO

ALTER TABLE dbo.section_versions
    ADD items NVARCHAR(MAX) NOT NULL CONSTRAINT df_section_versions_items DEFAULT N'[]';
GO

ALTER TABLE dbo.section_versions
    ADD CONSTRAINT ck_section_versions_items_json CHECK (ISJSON(items) = 1);
GO
