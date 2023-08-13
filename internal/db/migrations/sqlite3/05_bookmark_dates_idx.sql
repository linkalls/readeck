-- SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

CREATE INDEX bookmark_created_idx ON "bookmark" (created DESC);
CREATE INDEX bookmark_updated_idx ON "bookmark" (updated DESC);
