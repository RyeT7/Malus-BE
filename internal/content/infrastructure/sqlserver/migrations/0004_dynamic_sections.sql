ALTER TABLE dbo.sections
    ADD position INT NOT NULL CONSTRAINT df_sections_position DEFAULT 0;
GO

ALTER TABLE dbo.sections
    ADD draft_layout NVARCHAR(16) NOT NULL CONSTRAINT df_sections_draft_layout DEFAULT N'list';
GO

ALTER TABLE dbo.section_versions
    ADD layout NVARCHAR(16) NOT NULL CONSTRAINT df_section_versions_layout DEFAULT N'list';
GO

UPDATE dbo.sections
SET draft_layout = CASE kind WHEN N'biodata' THEN N'facts' WHEN N'workplan' THEN N'timeline' ELSE N'list' END;
GO

UPDATE v
SET v.layout = s.draft_layout
FROM dbo.section_versions AS v
JOIN dbo.sections AS s ON s.id = v.section_id;
GO

WITH ordered AS (
    SELECT position, ROW_NUMBER() OVER (
        ORDER BY CASE kind
            WHEN N'biodata' THEN 1
            WHEN N'strengths' THEN 2
            WHEN N'weaknesses' THEN 3
            WHEN N'workplan' THEN 4
            WHEN N'innovations' THEN 5
            WHEN N'proposed_changes' THEN 6
            WHEN N'why_me' THEN 7
            ELSE 8
        END, created_at) AS n
    FROM dbo.sections
)
UPDATE ordered SET position = n;
GO

ALTER TABLE dbo.sections
    ADD CONSTRAINT ck_sections_draft_layout CHECK (draft_layout IN (N'list', N'facts', N'timeline'));
GO

ALTER TABLE dbo.section_versions
    ADD CONSTRAINT ck_section_versions_layout CHECK (layout IN (N'list', N'facts', N'timeline'));
GO

ALTER TABLE dbo.sections DROP CONSTRAINT uq_sections_kind;
GO

ALTER TABLE dbo.sections ALTER COLUMN kind NVARCHAR(32) NULL;
GO
