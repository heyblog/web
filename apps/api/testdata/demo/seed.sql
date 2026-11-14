-- Load AGE planner hooks before executing any graph statements in this session.
LOAD 'age';

-- Local fictional directory data. Keep the whole import atomic, including AGE writes.
DO $seed$
DECLARE
    demo_source constant uuid := '019a5f00-0000-7000-8000-000000000001';
    existing_source uuid;
    source_site record;
    target_site record;
    destination integer;
BEGIN
    PERFORM pg_advisory_xact_lock(74821953);
    SELECT id INTO existing_source FROM directory.site_sources WHERE source_key = 'LOCAL_DEMO';
    IF existing_source IS NOT NULL THEN
        IF existing_source <> demo_source OR
            (SELECT count(*) FROM directory.site_origins WHERE source_id = demo_source) <> 36 THEN
            RAISE EXCEPTION 'LOCAL_DEMO marker conflicts with this dataset or is incomplete; inspect it manually';
        END IF;
        RAISE NOTICE 'LOCAL_DEMO already imported; preserving all existing edits';
        RETURN;
    END IF;

    CREATE TEMP TABLE heyblog_demo_sites ON COMMIT DROP AS
    SELECT n, ('019a5f10-0000-7000-8000-' || lpad(n::text, 12, '0'))::uuid AS id,
           name, '[演示数据] ' || summary AS summary, taxonomy_key,
           'blog-' || lpad(n::text, 2, '0') || '.heyblog.test' AS host,
           CASE WHEN n % 5 = 0 THEN '/blog' ELSE '/' END AS base_path,
           (ARRAY['astro', 'wordpress', 'hugo', 'hexo', 'typecho', 'ghost'])[(n - 1) % 6 + 1] AS program,
           (ARRAY['随笔', '教程', '开源', '城市漫步', '慢生活'])[(n - 1) % 5 + 1] AS tertiary_tag
      FROM (VALUES
        (1, '代码与咖啡', '记录前端开发、组件设计与开源实践。', 'computer/topic-computer-development'),
        (2, '像素散步', '用镜头收集街道、光影和城市的细节。', 'entertainment/topic-entertainment-photography'),
        (3, '山间来信', '周末徒步与长途旅行中的见闻。', 'travel/topic-travel-journal'),
        (4, '小锅日记', '一人食菜谱、烘焙实验和四季食材。', 'life/topic-life-food'),
        (5, '纸上星河', '阅读笔记与关于写作的日常练习。', 'humanities/topic-humanities-writing'),
        (6, '终端之外', '自动化工具与开发环境的折腾记录。', 'computer/topic-computer-tools'),
        (7, '慢速加载', '用于测试访问缓慢警告的虚拟技术博客。', 'computer/topic-computer-development'),
        (8, '生活的切片', '平凡日子的观察、整理与小结。', 'life/topic-life-other'),
        (9, '函数花园', '用直观例子探索数学和程序的关系。', 'science/topic-science-mathematics'),
        (10, '画室窗边', '绘画练习、展览笔记与视觉灵感。', 'humanities/topic-humanities-art'),
        (11, '构建日志', '从需求到上线的软件开发过程。', 'computer/topic-computer-development'),
        (12, '胶片星期天', '胶片摄影、相机使用和旅行照片。', 'entertainment/topic-entertainment-photography'),
        (13, '去远一点', '公共交通旅行与沿途的小城故事。', 'travel/topic-travel-journal'),
        (14, '旧时光缓存', '用于测试长期未更新警告的虚拟博客。', 'life/topic-life-other'),
        (15, '雨天书桌', '读书、摘录和写作练习。', 'humanities/topic-humanities-writing'),
        (16, '工具匣子', '常用软件体验与效率工具工作流。', 'computer/topic-computer-tools'),
        (17, '一页源码', '拆解小型项目并记录工程选择。', 'computer/topic-computer-development'),
        (18, '窗台植物志', '种植观察和生活里的缓慢变化。', 'life/topic-life-other'),
        (19, '概率茶馆', '概率直觉、统计故事和数学随想。', 'science/topic-science-mathematics'),
        (20, '色彩备忘录', '设计观察、配色练习与展览记录。', 'humanities/topic-humanities-art'),
        (21, '重试之后', '用于测试异常状态筛选的虚拟博客。', 'computer/topic-computer-development'),
        (22, '街角取景器', '街头摄影和城市步行的视觉日记。', 'entertainment/topic-entertainment-photography'),
        (23, '沿河向南', '骑行路线、旅行清单与路上见闻。', 'travel/topic-travel-journal'),
        (24, '厨房实验室', '家常菜改良和每周做饭记录。', 'life/topic-life-food'),
        (25, '句子收集站', '短篇随笔、阅读感受与文字片段。', 'humanities/topic-humanities-writing'),
        (26, '键盘捷径', '终端脚本与日常任务自动化。', 'computer/topic-computer-tools'),
        (27, '编译中的午后', '后端开发、测试与工程实践。', 'computer/topic-computer-development'),
        (28, '暂停营业的月亮', '用于测试带警告生活博客的虚拟站点。', 'life/topic-life-other'),
        (29, '几何便利店', '几何图形、数学谜题与学习笔记。', 'science/topic-science-mathematics'),
        (30, '留白之间', '视觉艺术、手作和日常审美。', 'humanities/topic-humanities-art'),
        (31, '独立小岛', '没有友链的公开博客，用于测试图谱孤立节点。', 'life/topic-life-other'),
        (32, '边界之外的长标题博客：关于代码、生活与偶尔出发的旅行', '用于测试长标题换行、空订阅源和孤立节点。', 'computer/topic-computer-development'),
        (33, '暂时隐身', '隐藏站点样本，用于测试可见性规则。', 'life/topic-life-other'),
        (34, '整理中的书房', '隐藏站点样本，模拟站点维护。', 'humanities/topic-humanities-writing'),
        (35, '已离开的渡口', '已移除站点样本，用于测试访问限制。', 'travel/topic-travel-journal'),
        (36, '归档的实验', '已移除站点样本，模拟停止运营。', 'computer/topic-computer-development')
      ) AS data(n, name, summary, taxonomy_key);

    IF EXISTS (
        SELECT 1 FROM heyblog_demo_sites AS demo
        WHERE NOT EXISTS (SELECT 1 FROM directory.tag_cascades AS cascade
            WHERE cascade.scope = 'SITE' AND cascade.taxonomy_key = demo.taxonomy_key
              AND cascade.is_enabled AND cascade.merged_into_id IS NULL)
    ) THEN
        RAISE EXCEPTION 'Required demo classifications are missing or disabled; run API migrations first';
    END IF;

    INSERT INTO directory.site_sources (id, source_key, name)
    VALUES (demo_source, 'LOCAL_DEMO', '本地虚拟博客演示集 v1');

    INSERT INTO directory.tags (name)
    SELECT name FROM unnest(ARRAY['随笔', '教程', '开源', '城市漫步', '慢生活',
        '演示：访问缓慢', '演示：长期未更新']) AS name
    ON CONFLICT (normalized_name) DO NOTHING;

    INSERT INTO directory.software_components (name, normalized_name, is_open_source)
    SELECT name, lower(name), true FROM unnest(ARRAY[
        'Astro', 'WordPress', 'Hugo', 'Hexo', 'Typecho', 'Ghost',
        'TypeScript', 'PHP', 'Go', 'JavaScript', 'Node.js']) AS name
    ON CONFLICT (normalized_name) DO NOTHING;

    INSERT INTO directory.sites (id, short_id, custom_id, name, normalized_host, base_path,
        summary, access_scope, visibility, visibility_reason, joined_at, updated_at, tag_cascade_id)
    SELECT demo.id, 'Demo' || lpad(n::text, 5, '0'), 'demo-' || lpad(n::text, 2, '0'),
           demo.name, host, base_path, summary,
           CASE WHEN n % 9 = 0 THEN 'CN_ONLY' WHEN n % 11 = 0 THEN 'GLOBAL_ONLY' ELSE 'ALL' END,
           CASE WHEN n <= 32 THEN 'VISIBLE' WHEN n <= 34 THEN 'HIDDEN' ELSE 'REMOVED' END,
           CASE WHEN n BETWEEN 33 AND 34 THEN '演示：站点维护中'
                WHEN n >= 35 THEN '演示：站点已停止运营' END,
           now() - n * interval '7 days', now() - n * interval '1 day', cascade.id
      FROM heyblog_demo_sites AS demo
      JOIN directory.tag_cascades AS cascade ON cascade.scope = 'SITE'
       AND cascade.taxonomy_key = demo.taxonomy_key ORDER BY n;

    INSERT INTO directory.site_origins (site_id, source_id, external_reference, metadata)
    SELECT id, demo_source, 'demo-' || lpad(n::text, 2, '0'),
           jsonb_build_object('dataset', 'heyblog-local-demo', 'version', 1, 'fictional', true)
      FROM heyblog_demo_sites;

    INSERT INTO directory.site_tags (site_id, tag_id, role, assignment_source, position)
    SELECT demo.id, tag.id, 'TERTIARY', 'IMPORTED', 1
      FROM heyblog_demo_sites AS demo JOIN directory.tags AS tag ON tag.name = demo.tertiary_tag;

    INSERT INTO directory.site_tags (site_id, tag_id, role, assignment_source, note)
    SELECT demo.id, tag.id, 'WARNING', 'IMPORTED', '虚拟状态，仅供页面筛选和提示测试'
      FROM heyblog_demo_sites AS demo JOIN directory.tags AS tag
        ON tag.name = CASE WHEN n % 14 = 0 THEN '演示：长期未更新' ELSE '演示：访问缓慢' END
     WHERE n % 7 = 0;

    INSERT INTO directory.site_software_components (site_id, component_id, role, evidence_source)
    SELECT demo.id, component.id, 'SITE_PROGRAM', 'IMPORTED'
      FROM heyblog_demo_sites AS demo JOIN directory.software_components AS component
        ON component.normalized_name = demo.program;

    INSERT INTO directory.site_software_components (site_id, component_id, role, evidence_source)
    SELECT demo.id, component.id, 'LANGUAGE', 'IMPORTED'
      FROM heyblog_demo_sites AS demo JOIN directory.software_components AS component
        ON component.normalized_name = CASE demo.program
            WHEN 'astro' THEN 'typescript' WHEN 'hugo' THEN 'go'
            WHEN 'wordpress' THEN 'php' WHEN 'typecho' THEN 'php' ELSE 'javascript' END;

    INSERT INTO directory.site_software_components (site_id, component_id, role, evidence_source)
    SELECT demo.id, component.id, 'RUNTIME', 'IMPORTED'
      FROM heyblog_demo_sites AS demo JOIN directory.software_components AS component
        ON component.normalized_name = 'node.js' WHERE demo.program IN ('astro', 'hexo', 'ghost');

    INSERT INTO directory.site_feeds (site_id, name, location_type, url_ref, url_key, format, is_default)
    SELECT id, '默认订阅', 'RELATIVE',
           rtrim(base_path, '/') || '/feed.xml', rtrim(base_path, '/') || '/feed.xml',
           CASE WHEN n % 2 = 0 THEN 'ATOM' ELSE 'RSS' END, true
      FROM heyblog_demo_sites WHERE n % 4 <> 0;

    INSERT INTO directory.site_feeds (site_id, name, location_type, url_ref, url_key, format)
    SELECT id, 'JSON Feed', 'RELATIVE',
           rtrim(base_path, '/') || '/feed.json', rtrim(base_path, '/') || '/feed.json', 'JSON'
      FROM heyblog_demo_sites WHERE n % 3 = 0 AND n % 4 <> 0;

    INSERT INTO directory.site_resources (site_id, kind, location_type, url_ref, url_key)
    SELECT id, 'LINK_PAGE', 'RELATIVE',
           rtrim(base_path, '/') || '/links', rtrim(base_path, '/') || '/links'
      FROM heyblog_demo_sites;

    INSERT INTO directory.site_resources (site_id, kind, location_type, url_ref, url_key)
    SELECT id, 'SITEMAP', 'RELATIVE',
           rtrim(base_path, '/') || '/sitemap.xml', rtrim(base_path, '/') || '/sitemap.xml'
      FROM heyblog_demo_sites WHERE n % 2 = 0;

    INSERT INTO directory.site_metrics (site_id, click_count, impression_count, response_count, query_count)
    SELECT id, n * 17, n * 149, n * 73, n * 91 FROM heyblog_demo_sites;

    FOR source_site IN SELECT * FROM heyblog_demo_sites WHERE n <= 30 ORDER BY n LOOP
        FOREACH destination IN ARRAY ARRAY[source_site.n % 30 + 1, (source_site.n + 4) % 30 + 1] LOOP
            SELECT * INTO STRICT target_site FROM heyblog_demo_sites WHERE n = destination;
            PERFORM directory.upsert_friend_link(source_site.id,
                'https://' || target_site.host || target_site.base_path, target_site.host, 'ACTIVE');
        END LOOP;
        IF source_site.n % 5 = 0 THEN
            SELECT * INTO STRICT target_site FROM heyblog_demo_sites WHERE n = source_site.n % 30 + 1;
            PERFORM directory.upsert_friend_link(target_site.id,
                'https://' || source_site.host || source_site.base_path, source_site.host, 'ACTIVE');
        END IF;
        IF source_site.n <= 2 THEN
            PERFORM directory.upsert_friend_link(source_site.id,
                'https://external-' || source_site.n || '.heyblog.test/',
                'external-' || source_site.n || '.heyblog.test', 'ACTIVE');
        END IF;
    END LOOP;

    RAISE NOTICE 'Imported LOCAL_DEMO v1: 36 sites (32 visible, 2 hidden, 2 removed), 36 feeds, 54 resources, 68 friend links';
END;
$seed$;
