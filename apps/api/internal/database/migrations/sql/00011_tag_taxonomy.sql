-- +goose Up

-- Reconcile databases that recorded migration 10 before review-draft columns were added.
ALTER TABLE directory.site_audits
    ADD COLUMN IF NOT EXISTS review_draft_snapshot jsonb,
    ADD COLUMN IF NOT EXISTS review_draft_revision bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS review_draft_updated_by uuid REFERENCES identity.users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS review_draft_updated_at timestamptz;

ALTER TABLE directory.tags
    ADD COLUMN system_key text, -- Stable SQLite taxonomy key for fixed entries; flexible tags use null.
    ADD COLUMN is_fixed boolean NOT NULL DEFAULT false; -- Whether the tag participates in a migration-owned fixed cascade.

ALTER TABLE directory.tags DROP CONSTRAINT tags_normalized_name_unique;

CREATE UNIQUE INDEX tags_system_key_unique_idx ON directory.tags (system_key)
WHERE system_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS directory.tag_seed_00011 (
    system_key text PRIMARY KEY,
    name text NOT NULL,
    description text NOT NULL,
    taxonomy_level smallint NOT NULL,
    parent_system_key text,
    sort_order smallint NOT NULL
);

INSERT INTO directory.tag_seed_00011 VALUES
('business', '商业', '经济、金融、商业、投资、管理、职业与职场', 1, NULL, 1),
('computer', '计算机', 'AI、软件、Web 开发、网络安全、云计算及计算机硬件（存储、CPU、GPU 等）', 1, NULL, 2),
('entertainment', '娱乐', '游戏、影视、体育、摄影、宠物、消费与兴趣爱好', 1, NULL, 3),
('health', '健康', '医疗、生物医药、心理健康、营养与公共卫生', 1, NULL, 4),
('humanities', '人文', '文学、哲学、历史、语言、宗教、文化、艺术与写作', 1, NULL, 5),
('life', '生活', '家庭、情感、人际、成长、感悟与日常记录', 1, NULL, 6),
('other', '其他', '无法明确归类或高度跨领域的内容', 1, NULL, 7),
('science', '科学', '电子信息、物理、生物、化学、材料等自然科学学科', 1, NULL, 8),
('society', '社会', '政治、法律、教育、传媒、环境及公共议题', 1, NULL, 9),
('travel', '旅行', '游记、行程、城市游览与户外活动', 1, NULL, 10),
('topic-business-career', '职业', '职业规划、求职招聘、职场经历、劳动关系与专业发展。', 2, 'business', 12),
('topic-business-consumption', '消费', '购物、价格、订阅、消费决策、服务体验与消费金融。', 2, 'business', 13),
('topic-business-entrepreneurship', '创业', '创业、独立开发、商业模式、产品经营与公司建设。', 2, 'business', 14),
('topic-business-finance', '财经', '经济、金融、投资、证券、信贷、货币与宏观市场。', 2, 'business', 15),
('topic-business-industry', '产业', '行业趋势、企业竞争、供应链、产业政策与科技产业。', 2, 'business', 16),
('topic-business-management', '管理', '企业管理、团队、组织、项目、决策与运营效率。', 2, 'business', 17),
('topic-business-marketing', '营销', '品牌、广告、推广、流量、销售及内容商业化。', 2, 'business', 18),
('topic-business-other', '其他', '商业相关但无法归入现有二级主题的内容。', 2, 'business', 19),
('topic-computer-ai', '人工智能', '人工智能、机器学习、大语言模型、智能体及生成式模型的原理、开发与应用。', 2, 'computer', 20),
('topic-computer-other', '其他', '计算机相关的其他内容，无法归入现有二级主题。', 2, 'computer', 21),
('topic-computer-data', '数据', '数据库、数据集、数据处理、存储、缓存、检索与分析技术。', 2, 'computer', 22),
('topic-computer-development', '开发', '编程语言、应用开发、前后端实现、测试、代码维护及软件工程实践。', 2, 'computer', 23),
('topic-computer-hardware', '硬件', '计算机硬件、芯片、处理器、嵌入式设备、外设及硬件维护。', 2, 'computer', 24),
('topic-computer-network', '网络', '网络协议、路由、域名、代理、通信、接入与网络基础设施。', 2, 'computer', 25),
('topic-computer-operations', '运维', '服务器、云平台、容器、部署、监控、备份、迁移与可靠性运维。', 2, 'computer', 26),
('topic-computer-cybersecurity', '网安', '网络安全、密码学、认证授权、漏洞、攻防、隐私与风险控制。', 2, 'computer', 27),
('topic-computer-operationsystems', '操作系统', '操作系统、内核、文件系统、运行时、驱动及系统资源管理。', 2, 'computer', 28),
('topic-computer-tools', '工具', '面向用户或开发者的软件、浏览器、编辑器、效率工具及使用体验。', 2, 'computer', 29),
('topic-entertainment-anime', '动漫', '动画、漫画、轻小说、同人及相关爱好文化。', 2, 'entertainment', 30),
('topic-entertainment-outdoors', '户外', '徒步、登山、骑行、潜水、漂流及户外探索。', 2, 'entertainment', 31),
('topic-entertainment-collection', '收藏', '模型、周边、纪念品、展览与个人收藏。', 2, 'entertainment', 32),
('topic-entertainment-film', '影视', '电影、电视剧、综艺、纪录片、观影与影视评论。', 2, 'entertainment', 33),
('topic-entertainment-games', '游戏', '电子游戏、桌游、游戏体验、评测、角色与玩法。', 2, 'entertainment', 34),
('topic-entertainment-music', '音乐', '音乐、歌曲、专辑、乐器、音乐制作与评论。', 2, 'entertainment', 35),
('topic-entertainment-performance', '演出', '戏剧、喜剧、音乐剧、展演及现场娱乐活动。', 2, 'entertainment', 36),
('topic-entertainment-photography', '摄影', '摄影实践、器材、照片、影像创作与展览。', 2, 'entertainment', 37),
('topic-entertainment-sports', '体育', '竞技体育、赛事、运动项目、选手与观赛体验。', 2, 'entertainment', 38),
('topic-entertainment-other', '其他', '娱乐相关但无法归入现有二级主题的内容。', 2, 'entertainment', 39),
('topic-health-fitness', '运动', '健身、游泳、跑步、身体训练与运动健康。', 2, 'health', 40),
('topic-health-medicine', '医疗', '疾病诊断、治疗、用药、手术、临床研究与医疗服务。', 2, 'health', 41),
('topic-health-nutrition', '营养', '营养、膳食、食品健康、喂养及代谢管理。', 2, 'health', 42),
('topic-health-public-health', '公卫', '公共卫生、流行病、传染病、健康风险与预防。', 2, 'health', 43),
('topic-health-wellness', '养生', '生活方式、保健、抗衰老、自我照护与健康习惯。', 2, 'health', 44),
('topic-health-other', '其他', '健康相关但无法归入现有二级主题的内容。', 2, 'health', 45),
('topic-humanities-art', '艺术', '绘画、设计、建筑、美术、博物馆及视觉艺术。', 2, 'humanities', 46),
('topic-humanities-culture', '文化', '民俗、地域文化、文化现象、文化遗产与文化评论。', 2, 'humanities', 47),
('topic-humanities-general', '综合', '无法稳定归入具体门类的综合性人文内容。', 2, 'humanities', 48),
('topic-humanities-history', '历史', '历史事件、人物、制度、遗迹、地方史与历史研究。', 2, 'humanities', 49),
('topic-humanities-language', '语言', '语言学、文字、语法、方言、翻译及人工语言。', 2, 'humanities', 50),
('topic-humanities-literature', '文学', '小说、诗歌、散文、文学作品、文学研究与批评。', 2, 'humanities', 51),
('topic-humanities-philosophy', '哲学', '哲学思想、认识论、伦理思辨、人生意义与思想史。', 2, 'humanities', 52),
('topic-humanities-religion', '宗教', '宗教传统、经典、信仰、神话及相关思想文化。', 2, 'humanities', 53),
('topic-humanities-writing', '写作', '写作方法、内容创作、出版、博客写作与文字表达。', 2, 'humanities', 54),
('topic-humanities-other', '其他', '人文相关但无法归入现有二级主题的内容。', 2, 'humanities', 55),
('topic-life-family', '家庭', '家庭生活、亲情、婚姻、育儿、照护与代际关系。', 2, 'life', 56),
('topic-life-food', '饮食', '饮食、烹饪、美食体验及日常餐饮生活。', 2, 'life', 57),
('topic-life-growth', '成长', '自我提升、目标、习惯、时间管理、学习与人生选择。', 2, 'life', 58),
('topic-life-home', '居家', '住房、家居、家务、车辆、维修及日常生活设施。', 2, 'life', 59),
('topic-life-pets', '宠物', '宠物饲养、动物陪伴、救助与日常观察。', 2, 'life', 60),
('topic-life-psychology', '心理', '认知、情绪、行为、动机、自我意识与心理体验。', 2, 'life', 61),
('topic-life-relationships', '情感', '恋爱、友情、人际交往、沟通、身份与情感体验。', 2, 'life', 62),
('topic-life-other', '其他', '生活相关但无法归入现有二级主题的内容。', 2, 'life', 63),
('topic-other-other', '其他', '一级领域与二级主题均无法明确归类的内容。', 2, 'other', 64),
('topic-science-aerospace', '航天', '天文、航空、航天器、空间探索及轨道科学。', 2, 'science', 65),
('topic-science-biology', '生物', '生物学、生命科学、遗传、细胞及生态系统研究。', 2, 'science', 66),
('topic-science-engineering', '工程', '电子、材料、机械、建筑及其他非计算机工程技术。', 2, 'science', 67),
('topic-science-environment', '环境', '气候、气象、地理、能源、生态与自然环境。', 2, 'science', 68),
('topic-science-mathematics', '数学', '数学、统计、概率、计算方法及形式化推理。', 2, 'science', 69),
('topic-science-physics', '物理', '物理学、力学、电磁学、光学、凝聚态及相关现象。', 2, 'science', 70),
('topic-science-research', '科研', '科学研究方法、实验、学术评价、科研工作与科学史。', 2, 'science', 71),
('topic-science-other', '其他', '科学相关但无法归入现有二级主题的内容。', 2, 'science', 72),
('topic-society-education', '教育', '学校教育、升学考试、教学、学习制度与人才培养。', 2, 'society', 73),
('topic-society-ethics', '伦理', '科技伦理、社会道德、偏见、公平与公共责任。', 2, 'society', 74),
('topic-society-law', '法律', '法律制度、司法、劳动法、知识产权、监管及权益保护。', 2, 'society', 75),
('topic-society-media', '传媒', '新闻、媒体、传播、舆论、平台治理与公共表达。', 2, 'society', 76),
('topic-society-politics', '政治', '政治制度、政府、国际关系、政策及公共治理。', 2, 'society', 77),
('topic-society-public', '公共', '社会结构、公共服务、城乡议题、群体处境与社会保障。', 2, 'society', 78),
('topic-society-other', '其他', '社会相关但无法归入现有二级主题的内容。', 2, 'society', 79),
('topic-travel-guide', '攻略', '目的地选择、行程规划、住宿、签证及旅行消费建议。', 2, 'travel', 80),
('topic-travel-journal', '游记', '旅行经历、城市见闻、景点记录与旅途感受。', 2, 'travel', 81),
('topic-travel-other', '其他', '旅行相关但无法归入现有二级主题的内容。', 2, 'travel', 82);

UPDATE directory.tags AS tag
   SET name = seed.name,
       normalized_name = lower(btrim(seed.name)),
       slug = seed.system_key,
       description = seed.description,
       system_key = seed.system_key,
       is_fixed = true
  FROM directory.tag_seed_00011 AS seed
 WHERE seed.name NOT IN ('其他', '综合')
   AND tag.normalized_name = lower(btrim(seed.name));

INSERT INTO directory.tags (name, normalized_name, slug, description, system_key, is_fixed)
SELECT seed.name, lower(btrim(seed.name)), seed.system_key, seed.description, seed.system_key, true
  FROM directory.tag_seed_00011 AS seed
 WHERE NOT EXISTS (
       SELECT 1 FROM directory.tags AS tag WHERE tag.system_key = seed.system_key
 );

CREATE UNIQUE INDEX tags_flexible_normalized_name_unique_idx
ON directory.tags (normalized_name)
WHERE NOT is_fixed AND merged_into_id IS NULL;

CREATE TABLE directory.tag_cascades (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 shared taxonomy path primary key.
    scope text NOT NULL, -- Object family using this path: SITE or ARTICLE.
    taxonomy_key text NOT NULL, -- Stable source path key independent of environment UUIDs.
    level1_tag_id uuid NOT NULL REFERENCES directory.tags(id) ON DELETE RESTRICT, -- Required fixed first-level tag.
    level2_tag_id uuid NOT NULL REFERENCES directory.tags(id) ON DELETE RESTRICT, -- Required fixed second-level tag.
    sort_order smallint NOT NULL, -- Stable SQLite taxonomy order used by clients and tie-breaking.
    is_enabled boolean NOT NULL DEFAULT true, -- Whether new assignments may select this fixed path.
    created_at timestamptz NOT NULL DEFAULT now(), -- Taxonomy path creation time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last taxonomy path update time maintained by trigger.
    CONSTRAINT tag_cascades_scope_check CHECK (scope IN ('SITE', 'ARTICLE')),
    CONSTRAINT tag_cascades_key_check CHECK (taxonomy_key ~ '^[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CONSTRAINT tag_cascades_tags_check CHECK (level1_tag_id <> level2_tag_id),
    CONSTRAINT tag_cascades_sort_check CHECK (sort_order > 0),
    CONSTRAINT tag_cascades_scope_key_unique UNIQUE (scope, taxonomy_key),
    CONSTRAINT tag_cascades_scope_pair_unique UNIQUE (scope, level1_tag_id, level2_tag_id),
    CONSTRAINT tag_cascades_scope_sort_unique UNIQUE (scope, sort_order)
);

CREATE INDEX tag_cascades_level1_idx ON directory.tag_cascades (scope, level1_tag_id, sort_order);
CREATE TRIGGER tag_cascades_touch_updated_at BEFORE UPDATE ON directory.tag_cascades
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

INSERT INTO directory.tag_cascades (
    scope, taxonomy_key, level1_tag_id, level2_tag_id, sort_order
)
SELECT scope.value,
       parent_seed.system_key || '/' || child_seed.system_key,
       parent_tag.id,
       child_tag.id,
       child_seed.sort_order
  FROM directory.tag_seed_00011 AS child_seed
  JOIN directory.tag_seed_00011 AS parent_seed ON parent_seed.system_key = child_seed.parent_system_key
  JOIN directory.tags AS parent_tag ON parent_tag.system_key = parent_seed.system_key
  JOIN directory.tags AS child_tag ON child_tag.system_key = child_seed.system_key
 CROSS JOIN (VALUES ('SITE'), ('ARTICLE')) AS scope(value)
 WHERE child_seed.taxonomy_level = 2;

DROP TABLE directory.tag_seed_00011;

ALTER TABLE directory.sites
    ADD COLUMN tag_cascade_id uuid; -- Required SITE taxonomy path.

UPDATE directory.sites
   SET tag_cascade_id = (
       SELECT id FROM directory.tag_cascades
        WHERE scope = 'SITE' AND taxonomy_key = 'other/topic-other-other'
   );

ALTER TABLE directory.sites
    ALTER COLUMN tag_cascade_id SET NOT NULL,
    ADD CONSTRAINT sites_tag_cascade_fkey
        FOREIGN KEY (tag_cascade_id) REFERENCES directory.tag_cascades(id) ON DELETE RESTRICT;

-- +goose StatementBegin
CREATE FUNCTION directory.ensure_site_cascade_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.tag_cascade_id IS NULL THEN
        SELECT id INTO NEW.tag_cascade_id
          FROM directory.tag_cascades
         WHERE scope = 'SITE' AND taxonomy_key = 'other/topic-other-other';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM directory.tag_cascades
         WHERE id = NEW.tag_cascade_id AND scope = 'SITE' AND is_enabled
    ) THEN
        RAISE EXCEPTION 'site tag cascade must reference an enabled SITE path';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM directory.site_tags AS assignment
          JOIN directory.tag_cascades AS cascade ON cascade.id = NEW.tag_cascade_id
         WHERE assignment.site_id = NEW.id
           AND assignment.role = 'TERTIARY'
           AND assignment.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'site tertiary tags must not repeat its fixed classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER sites_ensure_cascade_scope
BEFORE INSERT OR UPDATE OF tag_cascade_id ON directory.sites
FOR EACH ROW EXECUTE FUNCTION directory.ensure_site_cascade_scope();

DELETE FROM directory.site_tags WHERE role <> 'WARNING';
DROP INDEX directory.site_tags_primary_unique_idx;
ALTER TABLE directory.site_tags
    ALTER COLUMN role TYPE text, -- Assignment role: TERTIARY or WARNING.
    DROP CONSTRAINT site_tags_role_check,
    ADD COLUMN position smallint, -- Ordered tertiary slot from 1 through 20; warnings are unordered.
    ADD CONSTRAINT site_tags_role_check CHECK (role IN ('TERTIARY', 'WARNING')),
    ADD CONSTRAINT site_tags_position_check CHECK (
        (role = 'TERTIARY' AND position BETWEEN 1 AND 20)
        OR (role = 'WARNING' AND position IS NULL)
    );

CREATE UNIQUE INDEX site_tags_tertiary_position_unique_idx
ON directory.site_tags (site_id, position)
WHERE role = 'TERTIARY';

-- +goose StatementBegin
CREATE FUNCTION directory.ensure_site_tag_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.role = 'TERTIARY' AND EXISTS (
        SELECT 1
          FROM directory.sites AS site
          JOIN directory.tag_cascades AS cascade ON cascade.id = site.tag_cascade_id
         WHERE site.id = NEW.site_id
           AND NEW.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'site tertiary tags must not repeat its fixed classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER site_tags_ensure_assignment
BEFORE INSERT OR UPDATE ON directory.site_tags
FOR EACH ROW EXECUTE FUNCTION directory.ensure_site_tag_assignment();

CREATE TABLE content.articles (
    id uuid PRIMARY KEY DEFAULT uuidv7(), -- UUIDv7 internal article primary key.
    site_id uuid NOT NULL REFERENCES directory.sites(id) ON DELETE CASCADE, -- Blog site publishing the article.
    location_type text NOT NULL, -- Location storage mode: RELATIVE or EXTERNAL.
    url_ref text, -- Root-relative same-host article location without fragment.
    external_url text, -- Absolute HTTP or HTTPS location for an externally hosted article.
    url_key text NOT NULL, -- Normalized per-site article identity key.
    title text, -- Optional article title supplied by the source.
    published_at timestamptz, -- Optional source publication time.
    tag_cascade_id uuid NOT NULL REFERENCES directory.tag_cascades(id) ON DELETE RESTRICT, -- Required ARTICLE taxonomy path.
    created_at timestamptz NOT NULL DEFAULT now(), -- Article discovery time.
    updated_at timestamptz NOT NULL DEFAULT now(), -- Last article metadata update time maintained by trigger.
    CONSTRAINT articles_location_check CHECK (
        (location_type = 'RELATIVE' AND url_ref ~ '^/' AND url_ref !~ '#' AND external_url IS NULL AND url_key = url_ref)
        OR (location_type = 'EXTERNAL' AND url_ref IS NULL AND external_url ~ '^https?://' AND external_url !~ '#' AND url_key = external_url)
    ),
    CONSTRAINT articles_title_check CHECK (title IS NULL OR btrim(title) <> ''),
    CONSTRAINT articles_site_url_unique UNIQUE (site_id, url_key)
);

CREATE INDEX articles_site_published_idx ON content.articles (site_id, published_at DESC NULLS LAST, id);
CREATE TRIGGER articles_touch_updated_at BEFORE UPDATE ON content.articles
FOR EACH ROW EXECUTE FUNCTION directory.touch_updated_at();

-- +goose StatementBegin
CREATE FUNCTION content.ensure_article_cascade_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM directory.tag_cascades
         WHERE id = NEW.tag_cascade_id AND scope = 'ARTICLE' AND is_enabled
    ) THEN
        RAISE EXCEPTION 'article tag cascade must reference an enabled ARTICLE path';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM content.article_tags AS assignment
          JOIN directory.tag_cascades AS cascade ON cascade.id = NEW.tag_cascade_id
         WHERE assignment.article_id = NEW.id
           AND assignment.role = 'TERTIARY'
           AND assignment.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'article tertiary tags must not repeat its fixed classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER articles_ensure_cascade_scope
BEFORE INSERT OR UPDATE OF tag_cascade_id ON content.articles
FOR EACH ROW EXECUTE FUNCTION content.ensure_article_cascade_scope();

CREATE TABLE content.article_tags (
    article_id uuid NOT NULL REFERENCES content.articles(id) ON DELETE CASCADE, -- Article receiving the shared tag assignment.
    tag_id uuid NOT NULL REFERENCES directory.tags(id) ON DELETE RESTRICT, -- Assigned shared tag dictionary entry.
    role text NOT NULL, -- Assignment role: TERTIARY or WARNING.
    assignment_source text NOT NULL DEFAULT 'MANUAL', -- Evidence source: MANUAL, IMPORTED, or SYSTEM.
    position smallint, -- Ordered tertiary slot from 1 through 20; warnings are unordered.
    note text, -- Optional target-specific warning or assignment note.
    created_at timestamptz NOT NULL DEFAULT now(), -- Assignment creation time.
    PRIMARY KEY (article_id, tag_id),
    CONSTRAINT article_tags_role_check CHECK (role IN ('TERTIARY', 'WARNING')),
    CONSTRAINT article_tags_source_check CHECK (assignment_source IN ('MANUAL', 'IMPORTED', 'SYSTEM')),
    CONSTRAINT article_tags_position_check CHECK (
        (role = 'TERTIARY' AND position BETWEEN 1 AND 20)
        OR (role = 'WARNING' AND position IS NULL)
    ),
    CONSTRAINT article_tags_note_check CHECK (note IS NULL OR btrim(note) <> '')
);

CREATE UNIQUE INDEX article_tags_tertiary_position_unique_idx
ON content.article_tags (article_id, position)
WHERE role = 'TERTIARY';
CREATE INDEX article_tags_tag_idx ON content.article_tags (tag_id, article_id);

-- +goose StatementBegin
CREATE FUNCTION content.ensure_article_tag_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.role = 'TERTIARY' AND EXISTS (
        SELECT 1
          FROM content.articles AS article
          JOIN directory.tag_cascades AS cascade ON cascade.id = article.tag_cascade_id
         WHERE article.id = NEW.article_id
           AND NEW.tag_id IN (cascade.level1_tag_id, cascade.level2_tag_id)
    ) THEN
        RAISE EXCEPTION 'article tertiary tags must not repeat its fixed classification';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER article_tags_ensure_assignment
BEFORE INSERT OR UPDATE ON content.article_tags
FOR EACH ROW EXECUTE FUNCTION content.ensure_article_tag_assignment();

-- +goose StatementBegin
CREATE FUNCTION directory.migrate_site_audit_tag_snapshot(snapshot jsonb)
RETURNS jsonb
LANGUAGE sql
STABLE
AS $$
    WITH fallback AS (
        SELECT cascade.id, cascade.taxonomy_key,
               level1.id AS level1_id, level1.name AS level1_name, level1.slug AS level1_slug,
               level2.id AS level2_id, level2.name AS level2_name, level2.slug AS level2_slug
          FROM directory.tag_cascades AS cascade
          JOIN directory.tags AS level1 ON level1.id = cascade.level1_tag_id
          JOIN directory.tags AS level2 ON level2.id = cascade.level2_tag_id
         WHERE cascade.scope = 'SITE' AND cascade.taxonomy_key = 'other/topic-other-other'
    ), old_tags AS (
        SELECT element
          FROM jsonb_array_elements(COALESCE(snapshot->'tags', '[]'::jsonb)) AS element
    )
    SELECT CASE WHEN snapshot IS NULL THEN NULL ELSE
		(snapshot - 'tags') || jsonb_build_object(
			'tag_cascade_id', fallback.id::text,
            'classification', jsonb_build_object(
                'id', fallback.id::text,
                'taxonomy_key', fallback.taxonomy_key,
				'level1', jsonb_build_object('id', fallback.level1_id::text, 'name', fallback.level1_name, 'slug', fallback.level1_slug, 'role', 'PRIMARY', 'level', 1),
				'level2', jsonb_build_object('id', fallback.level2_id::text, 'name', fallback.level2_name, 'slug', fallback.level2_slug, 'role', 'SECONDARY', 'level', 2, 'parent_id', fallback.level1_id::text)
            ),
			'tags', jsonb_build_array(
				jsonb_build_object('id', fallback.level1_id::text, 'name', fallback.level1_name, 'slug', fallback.level1_slug, 'role', 'PRIMARY', 'level', 1),
				jsonb_build_object('id', fallback.level2_id::text, 'name', fallback.level2_name, 'slug', fallback.level2_slug, 'role', 'SECONDARY', 'level', 2, 'parent_id', fallback.level1_id::text)
			) || COALESCE((
				SELECT jsonb_agg((element - 'role') || jsonb_build_object(
					'role', CASE WHEN element->>'role' = 'WARNING' THEN 'WARNING' ELSE 'TERTIARY' END,
					'level', 3
				)) FROM old_tags
			), '[]'::jsonb)
        ) END
      FROM fallback;
$$;
-- +goose StatementEnd

UPDATE directory.site_audits
   SET base_snapshot = directory.migrate_site_audit_tag_snapshot(base_snapshot),
       proposed_snapshot = directory.migrate_site_audit_tag_snapshot(proposed_snapshot),
       review_draft_snapshot = directory.migrate_site_audit_tag_snapshot(review_draft_snapshot),
       final_snapshot = directory.migrate_site_audit_tag_snapshot(final_snapshot);

DROP FUNCTION directory.migrate_site_audit_tag_snapshot(jsonb);

COMMENT ON TABLE directory.tag_cascades IS 'Fixed first- and second-level taxonomy paths shared by site and article assignment scopes.';
COMMENT ON COLUMN directory.tag_cascades.id IS 'UUIDv7 shared taxonomy path primary key.';
COMMENT ON COLUMN directory.tag_cascades.scope IS 'Object family using this path: SITE or ARTICLE.';
COMMENT ON COLUMN directory.tag_cascades.taxonomy_key IS 'Stable source path key independent of environment UUIDs.';
COMMENT ON COLUMN directory.tag_cascades.level1_tag_id IS 'Required fixed first-level tag.';
COMMENT ON COLUMN directory.tag_cascades.level2_tag_id IS 'Required fixed second-level tag.';
COMMENT ON COLUMN directory.tag_cascades.sort_order IS 'Stable SQLite taxonomy order used by clients and tie-breaking.';
COMMENT ON COLUMN directory.tag_cascades.is_enabled IS 'Whether new assignments may select this fixed path.';
COMMENT ON COLUMN directory.tag_cascades.created_at IS 'Taxonomy path creation time.';
COMMENT ON COLUMN directory.tag_cascades.updated_at IS 'Last taxonomy path update time maintained by trigger.';

COMMENT ON TABLE content.articles IS 'Article identity and metadata prepared for future ingestion without storing article bodies.';
COMMENT ON COLUMN content.articles.id IS 'UUIDv7 internal article primary key.';
COMMENT ON COLUMN content.articles.site_id IS 'Blog site publishing the article.';
COMMENT ON COLUMN content.articles.location_type IS 'Location storage mode: RELATIVE or EXTERNAL.';
COMMENT ON COLUMN content.articles.url_ref IS 'Root-relative same-host article location without fragment.';
COMMENT ON COLUMN content.articles.external_url IS 'Absolute HTTP or HTTPS location for an externally hosted article.';
COMMENT ON COLUMN content.articles.url_key IS 'Normalized per-site article identity key.';
COMMENT ON COLUMN content.articles.title IS 'Optional article title supplied by the source.';
COMMENT ON COLUMN content.articles.published_at IS 'Optional source publication time.';
COMMENT ON COLUMN content.articles.tag_cascade_id IS 'Required ARTICLE taxonomy path.';
COMMENT ON COLUMN content.articles.created_at IS 'Article discovery time.';
COMMENT ON COLUMN content.articles.updated_at IS 'Last article metadata update time maintained by trigger.';

COMMENT ON TABLE content.article_tags IS 'Optional ordered tertiary and warning tag assignments for articles.';
COMMENT ON COLUMN content.article_tags.article_id IS 'Article receiving the shared tag assignment.';
COMMENT ON COLUMN content.article_tags.tag_id IS 'Assigned shared tag dictionary entry.';
COMMENT ON COLUMN content.article_tags.role IS 'Assignment role: TERTIARY or WARNING.';
COMMENT ON COLUMN content.article_tags.assignment_source IS 'Evidence source: MANUAL, IMPORTED, or SYSTEM.';
COMMENT ON COLUMN content.article_tags.position IS 'Ordered tertiary slot from 1 through 20; warnings are unordered.';
COMMENT ON COLUMN content.article_tags.note IS 'Optional target-specific warning or assignment note.';
COMMENT ON COLUMN content.article_tags.created_at IS 'Assignment creation time.';

COMMENT ON COLUMN directory.tags.system_key IS 'Stable SQLite taxonomy key for fixed entries; flexible tags use null.';
COMMENT ON COLUMN directory.tags.is_fixed IS 'Whether the tag participates in a migration-owned fixed cascade.';
COMMENT ON COLUMN directory.sites.tag_cascade_id IS 'Required SITE taxonomy path.';
COMMENT ON COLUMN directory.site_tags.position IS 'Ordered tertiary slot from 1 through 20; warnings are unordered.';
COMMENT ON COLUMN directory.site_tags.role IS 'Assignment role: TERTIARY or WARNING.';

COMMENT ON FUNCTION directory.ensure_site_cascade_scope() IS 'Enforces enabled SITE cascade scope and prevents tertiary classification duplication.';
COMMENT ON FUNCTION directory.ensure_site_tag_assignment() IS 'Rejects site tertiary tags that duplicate the selected fixed classification.';
COMMENT ON FUNCTION content.ensure_article_cascade_scope() IS 'Enforces enabled ARTICLE cascade scope and prevents tertiary classification duplication.';
COMMENT ON FUNCTION content.ensure_article_tag_assignment() IS 'Rejects article tertiary tags that duplicate the selected fixed classification.';

COMMENT ON TRIGGER tag_cascades_touch_updated_at ON directory.tag_cascades IS 'Maintains taxonomy path update timestamps.';
COMMENT ON TRIGGER sites_ensure_cascade_scope ON directory.sites IS 'Enforces the SITE cascade invariant.';
COMMENT ON TRIGGER site_tags_ensure_assignment ON directory.site_tags IS 'Enforces site tertiary classification exclusion.';
COMMENT ON TRIGGER articles_touch_updated_at ON content.articles IS 'Maintains article update timestamps.';
COMMENT ON TRIGGER articles_ensure_cascade_scope ON content.articles IS 'Enforces the ARTICLE cascade invariant.';
COMMENT ON TRIGGER article_tags_ensure_assignment ON content.article_tags IS 'Enforces article tertiary classification exclusion.';

GRANT SELECT ON directory.tag_cascades TO api_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON content.articles, content.article_tags TO api_runtime;

-- +goose Down

DROP TABLE IF EXISTS content.article_tags;
DROP TABLE IF EXISTS content.articles;
DROP FUNCTION IF EXISTS content.ensure_article_tag_assignment();
DROP FUNCTION IF EXISTS content.ensure_article_cascade_scope();

DROP TRIGGER IF EXISTS site_tags_ensure_assignment ON directory.site_tags;
DROP FUNCTION IF EXISTS directory.ensure_site_tag_assignment();
DROP INDEX IF EXISTS directory.site_tags_tertiary_position_unique_idx;
DROP INDEX IF EXISTS directory.site_tags_primary_unique_idx;
DELETE FROM directory.site_tags WHERE role = 'TERTIARY';
ALTER TABLE directory.site_tags
    DROP CONSTRAINT IF EXISTS site_tags_position_check,
    DROP CONSTRAINT IF EXISTS site_tags_role_check,
    DROP COLUMN IF EXISTS position,
    ADD CONSTRAINT site_tags_role_check CHECK (role IN ('PRIMARY', 'SECONDARY', 'WARNING'));
CREATE UNIQUE INDEX site_tags_primary_unique_idx ON directory.site_tags (site_id) WHERE role = 'PRIMARY';

DROP TRIGGER IF EXISTS sites_ensure_cascade_scope ON directory.sites;
DROP FUNCTION IF EXISTS directory.ensure_site_cascade_scope();
ALTER TABLE directory.sites DROP COLUMN IF EXISTS tag_cascade_id;
DROP TABLE IF EXISTS directory.tag_cascades;

DROP INDEX IF EXISTS directory.tags_flexible_normalized_name_unique_idx;
DROP INDEX IF EXISTS directory.tags_system_key_unique_idx;
ALTER TABLE directory.tags DROP COLUMN IF EXISTS is_fixed, DROP COLUMN IF EXISTS system_key;
