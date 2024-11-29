-- SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

ALTER TABLE "bookmark" ADD COLUMN percent_complete double NOT NULL DEFAULT 0;

UPDATE "bookmark" SET percent_complete = 1 WHERE is_archived;
