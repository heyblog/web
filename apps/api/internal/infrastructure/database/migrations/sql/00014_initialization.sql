-- +goose Up

SET LOCAL check_function_bodies = false;

ALTER TABLE ONLY content.announcement_revisions
    ADD CONSTRAINT announcement_revisions_announcement_id_fkey FOREIGN KEY (announcement_id) REFERENCES content.announcements(id) ON DELETE RESTRICT;

ALTER TABLE ONLY content.announcement_revisions
    ADD CONSTRAINT announcement_revisions_changed_by_fkey FOREIGN KEY (changed_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY content.announcement_revisions
    ADD CONSTRAINT announcement_revisions_published_by_fkey FOREIGN KEY (published_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY content.announcements
    ADD CONSTRAINT announcements_archived_by_fkey FOREIGN KEY (archived_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY content.announcements
    ADD CONSTRAINT announcements_created_by_fkey FOREIGN KEY (created_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY content.announcements
    ADD CONSTRAINT announcements_published_by_fkey FOREIGN KEY (published_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY content.announcements
    ADD CONSTRAINT announcements_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY content.article_tags
    ADD CONSTRAINT article_tags_article_id_fkey FOREIGN KEY (article_id) REFERENCES content.articles(id) ON DELETE CASCADE;

ALTER TABLE ONLY content.article_tags
    ADD CONSTRAINT article_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES directory.tags(id) ON DELETE RESTRICT;

ALTER TABLE ONLY content.articles
    ADD CONSTRAINT articles_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY content.articles
    ADD CONSTRAINT articles_tag_cascade_id_fkey FOREIGN KEY (tag_cascade_id) REFERENCES directory.tag_cascades(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.owner_friend_link_requests
    ADD CONSTRAINT owner_friend_link_requests_audit_id_fkey FOREIGN KEY (audit_id) REFERENCES directory.site_audits(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.owner_friend_link_requests
    ADD CONSTRAINT owner_friend_link_requests_source_site_id_fkey FOREIGN KEY (source_site_id) REFERENCES directory.sites(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.owner_friend_link_requests
    ADD CONSTRAINT owner_friend_link_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_audits
    ADD CONSTRAINT site_audits_review_draft_updated_by_fkey FOREIGN KEY (review_draft_updated_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_audits
    ADD CONSTRAINT site_audits_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_audits
    ADD CONSTRAINT site_audits_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.site_audits
    ADD CONSTRAINT site_audits_source_site_id_fkey FOREIGN KEY (source_site_id) REFERENCES directory.sites(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.site_audits
    ADD CONSTRAINT site_audits_submitter_user_id_fkey FOREIGN KEY (submitter_user_id) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_claims
    ADD CONSTRAINT site_claims_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_claims
    ADD CONSTRAINT site_claims_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id);

ALTER TABLE ONLY directory.site_claims
    ADD CONSTRAINT site_claims_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_feeds
    ADD CONSTRAINT site_feeds_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_icons
    ADD CONSTRAINT site_icons_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_metric_events
    ADD CONSTRAINT site_metric_events_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_metrics
    ADD CONSTRAINT site_metrics_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_origins
    ADD CONSTRAINT site_origins_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_origins
    ADD CONSTRAINT site_origins_source_id_fkey FOREIGN KEY (source_id) REFERENCES directory.site_sources(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.site_ownership_events
    ADD CONSTRAINT site_ownership_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_ownership_events
    ADD CONSTRAINT site_ownership_events_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id);

ALTER TABLE ONLY directory.site_ownership_events
    ADD CONSTRAINT site_ownership_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_ownerships
    ADD CONSTRAINT site_ownerships_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id);

ALTER TABLE ONLY directory.site_ownerships
    ADD CONSTRAINT site_ownerships_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_ownerships
    ADD CONSTRAINT site_ownerships_verified_claim_id_fkey FOREIGN KEY (verified_claim_id) REFERENCES directory.site_claims(id);

ALTER TABLE ONLY directory.site_resources
    ADD CONSTRAINT site_resources_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_software_components
    ADD CONSTRAINT site_software_components_component_id_fkey FOREIGN KEY (component_id) REFERENCES directory.software_components(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.site_software_components
    ADD CONSTRAINT site_software_components_identified_by_fkey FOREIGN KEY (identified_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY directory.site_software_components
    ADD CONSTRAINT site_software_components_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_tags
    ADD CONSTRAINT site_tags_site_id_fkey FOREIGN KEY (site_id) REFERENCES directory.sites(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.site_tags
    ADD CONSTRAINT site_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES directory.tags(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.sites
    ADD CONSTRAINT sites_tag_cascade_fkey FOREIGN KEY (tag_cascade_id) REFERENCES directory.tag_cascades(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.software_component_dependencies
    ADD CONSTRAINT software_component_dependencies_component_id_fkey FOREIGN KEY (component_id) REFERENCES directory.software_components(id) ON DELETE CASCADE;

ALTER TABLE ONLY directory.software_component_dependencies
    ADD CONSTRAINT software_component_dependencies_dependency_component_id_fkey FOREIGN KEY (dependency_component_id) REFERENCES directory.software_components(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.tag_cascades
    ADD CONSTRAINT tag_cascades_level1_tag_id_fkey FOREIGN KEY (level1_tag_id) REFERENCES directory.tags(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.tag_cascades
    ADD CONSTRAINT tag_cascades_level2_tag_id_fkey FOREIGN KEY (level2_tag_id) REFERENCES directory.tags(id) ON DELETE RESTRICT;

ALTER TABLE ONLY directory.tag_cascades
    ADD CONSTRAINT tag_cascades_merged_into_id_fkey FOREIGN KEY (merged_into_id) REFERENCES directory.tag_cascades(id) ON DELETE RESTRICT DEFERRABLE;

ALTER TABLE ONLY directory.tag_identity_aliases
    ADD CONSTRAINT tag_identity_aliases_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES directory.tags(id) ON DELETE RESTRICT;

ALTER TABLE ONLY identity.api_client_scopes
    ADD CONSTRAINT api_client_scopes_client_id_fkey FOREIGN KEY (client_id) REFERENCES identity.api_clients(id) ON DELETE CASCADE;

ALTER TABLE ONLY identity.api_clients
    ADD CONSTRAINT api_clients_created_by_fkey FOREIGN KEY (created_by) REFERENCES identity.users(id);

ALTER TABLE ONLY identity.api_keys
    ADD CONSTRAINT api_keys_client_id_fkey FOREIGN KEY (client_id) REFERENCES identity.api_clients(id);

ALTER TABLE ONLY identity.api_keys
    ADD CONSTRAINT api_keys_created_by_fkey FOREIGN KEY (created_by) REFERENCES identity.users(id);

ALTER TABLE ONLY identity.api_keys
    ADD CONSTRAINT api_keys_revoked_by_fkey FOREIGN KEY (revoked_by) REFERENCES identity.users(id);

ALTER TABLE ONLY identity.api_keys
    ADD CONSTRAINT api_keys_rotated_from_id_fkey FOREIGN KEY (rotated_from_id) REFERENCES identity.api_keys(id) DEFERRABLE;

ALTER TABLE ONLY identity.email_verification_codes
    ADD CONSTRAINT email_verification_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY identity.oauth_identities
    ADD CONSTRAINT oauth_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY identity.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY identity.user_management_permissions
    ADD CONSTRAINT user_management_permissions_granted_by_fkey FOREIGN KEY (granted_by) REFERENCES identity.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY identity.user_management_permissions
    ADD CONSTRAINT user_management_permissions_user_id_fkey FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;

-- Initial data is expressed with logical identities; UUIDs are generated per database.

-- +goose StatementBegin

DO $seed$
DECLARE
    data jsonb := $data${
"directory.tags": [
{"id": "$id:tag:产业", "name": "产业", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "行业趋势、企业竞争、供应链、产业政策与科技产业。", "normalized_name": "产业"},
{"id": "$id:tag:人工智能", "name": "人工智能", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "人工智能、机器学习、大语言模型、智能体及生成式模型的原理、开发与应用。", "normalized_name": "人工智能"},
{"id": "$id:tag:人文", "name": "人文", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "文学、哲学、历史、语言、宗教、文化、艺术与写作", "normalized_name": "人文"},
{"id": "$id:tag:传媒", "name": "传媒", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "新闻、媒体、传播、舆论、平台治理与公共表达。", "normalized_name": "传媒"},
{"id": "$id:tag:伦理", "name": "伦理", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "科技伦理、社会道德、偏见、公平与公共责任。", "normalized_name": "伦理"},
{"id": "$id:tag:体育", "name": "体育", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "竞技体育、赛事、运动项目、选手与观赛体验。", "normalized_name": "体育"},
{"id": "$id:tag:健康", "name": "健康", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "医疗、生物医药、心理健康、营养与公共卫生", "normalized_name": "健康"},
{"id": "$id:tag:公共", "name": "公共", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "社会结构、公共服务、城乡议题、群体处境与社会保障。", "normalized_name": "公共"},
{"id": "$id:tag:公卫", "name": "公卫", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "公共卫生、流行病、传染病、健康风险与预防。", "normalized_name": "公卫"},
{"id": "$id:tag:其他", "name": "其他", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "无法明确归类或高度跨领域的内容", "normalized_name": "其他"},
{"id": "$id:tag:养生", "name": "养生", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "生活方式、保健、抗衰老、自我照护与健康习惯。", "normalized_name": "养生"},
{"id": "$id:tag:写作", "name": "写作", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "写作方法、内容创作、出版、博客写作与文字表达。", "normalized_name": "写作"},
{"id": "$id:tag:创业", "name": "创业", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "创业、独立开发、商业模式、产品经营与公司建设。", "normalized_name": "创业"},
{"id": "$id:tag:动漫", "name": "动漫", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "动画、漫画、轻小说、同人及相关爱好文化。", "normalized_name": "动漫"},
{"id": "$id:tag:医疗", "name": "医疗", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "疾病诊断、治疗、用药、手术、临床研究与医疗服务。", "normalized_name": "医疗"},
{"id": "$id:tag:历史", "name": "历史", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "历史事件、人物、制度、遗迹、地方史与历史研究。", "normalized_name": "历史"},
{"id": "$id:tag:哲学", "name": "哲学", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "哲学思想、认识论、伦理思辨、人生意义与思想史。", "normalized_name": "哲学"},
{"id": "$id:tag:商业", "name": "商业", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "经济、金融、商业、投资、管理、职业与职场", "normalized_name": "商业"},
{"id": "$id:tag:娱乐", "name": "娱乐", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "游戏、影视、体育、摄影、宠物、消费与兴趣爱好", "normalized_name": "娱乐"},
{"id": "$id:tag:宗教", "name": "宗教", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "宗教传统、经典、信仰、神话及相关思想文化。", "normalized_name": "宗教"},
{"id": "$id:tag:宠物", "name": "宠物", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "宠物饲养、动物陪伴、救助与日常观察。", "normalized_name": "宠物"},
{"id": "$id:tag:家庭", "name": "家庭", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "家庭生活、亲情、婚姻、育儿、照护与代际关系。", "normalized_name": "家庭"},
{"id": "$id:tag:居家", "name": "居家", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "住房、家居、家务、车辆、维修及日常生活设施。", "normalized_name": "居家"},
{"id": "$id:tag:工具", "name": "工具", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "面向用户或开发者的软件、浏览器、编辑器、效率工具及使用体验。", "normalized_name": "工具"},
{"id": "$id:tag:工程", "name": "工程", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "电子、材料、机械、建筑及其他非计算机工程技术。", "normalized_name": "工程"},
{"id": "$id:tag:开发", "name": "开发", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "编程语言、应用开发、前后端实现、测试、代码维护及软件工程实践。", "normalized_name": "开发"},
{"id": "$id:tag:影视", "name": "影视", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "电影、电视剧、综艺、纪录片、观影与影视评论。", "normalized_name": "影视"},
{"id": "$id:tag:心理", "name": "心理", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "认知、情绪、行为、动机、自我意识与心理体验。", "normalized_name": "心理"},
{"id": "$id:tag:情感", "name": "情感", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "恋爱、友情、人际交往、沟通、身份与情感体验。", "normalized_name": "情感"},
{"id": "$id:tag:成长", "name": "成长", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "自我提升、目标、习惯、时间管理、学习与人生选择。", "normalized_name": "成长"},
{"id": "$id:tag:户外", "name": "户外", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "徒步、登山、骑行、潜水、漂流及户外探索。", "normalized_name": "户外"},
{"id": "$id:tag:摄影", "name": "摄影", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "摄影实践、器材、照片、影像创作与展览。", "normalized_name": "摄影"},
{"id": "$id:tag:操作系统", "name": "操作系统", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "操作系统、内核、文件系统、运行时、驱动及系统资源管理。", "normalized_name": "操作系统"},
{"id": "$id:tag:收藏", "name": "收藏", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "模型、周边、纪念品、展览与个人收藏。", "normalized_name": "收藏"},
{"id": "$id:tag:攻略", "name": "攻略", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "目的地选择、行程规划、住宿、签证及旅行消费建议。", "normalized_name": "攻略"},
{"id": "$id:tag:政治", "name": "政治", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "政治制度、政府、国际关系、政策及公共治理。", "normalized_name": "政治"},
{"id": "$id:tag:教育", "name": "教育", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "学校教育、升学考试、教学、学习制度与人才培养。", "normalized_name": "教育"},
{"id": "$id:tag:数学", "name": "数学", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "数学、统计、概率、计算方法及形式化推理。", "normalized_name": "数学"},
{"id": "$id:tag:数据", "name": "数据", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "数据库、数据集、数据处理、存储、缓存、检索与分析技术。", "normalized_name": "数据"},
{"id": "$id:tag:文化", "name": "文化", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "民俗、地域文化、文化现象、文化遗产与文化评论。", "normalized_name": "文化"},
{"id": "$id:tag:文学", "name": "文学", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "小说、诗歌、散文、文学作品、文学研究与批评。", "normalized_name": "文学"},
{"id": "$id:tag:旅行", "name": "旅行", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "游记、行程、城市游览与户外活动", "normalized_name": "旅行"},
{"id": "$id:tag:法律", "name": "法律", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "法律制度、司法、劳动法、知识产权、监管及权益保护。", "normalized_name": "法律"},
{"id": "$id:tag:消费", "name": "消费", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "购物、价格、订阅、消费决策、服务体验与消费金融。", "normalized_name": "消费"},
{"id": "$id:tag:游戏", "name": "游戏", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "电子游戏、桌游、游戏体验、评测、角色与玩法。", "normalized_name": "游戏"},
{"id": "$id:tag:游记", "name": "游记", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "旅行经历、城市见闻、景点记录与旅途感受。", "normalized_name": "游记"},
{"id": "$id:tag:演出", "name": "演出", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "戏剧、喜剧、音乐剧、展演及现场娱乐活动。", "normalized_name": "演出"},
{"id": "$id:tag:物理", "name": "物理", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "物理学、力学、电磁学、光学、凝聚态及相关现象。", "normalized_name": "物理"},
{"id": "$id:tag:环境", "name": "环境", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "气候、气象、地理、能源、生态与自然环境。", "normalized_name": "环境"},
{"id": "$id:tag:生活", "name": "生活", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "家庭、情感、人际、成长、感悟与日常记录", "normalized_name": "生活"},
{"id": "$id:tag:生物", "name": "生物", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "生物学、生命科学、遗传、细胞及生态系统研究。", "normalized_name": "生物"},
{"id": "$id:tag:硬件", "name": "硬件", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "计算机硬件、芯片、处理器、嵌入式设备、外设及硬件维护。", "normalized_name": "硬件"},
{"id": "$id:tag:社会", "name": "社会", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "政治、法律、教育、传媒、环境及公共议题", "normalized_name": "社会"},
{"id": "$id:tag:科学", "name": "科学", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "电子信息、物理、生物、化学、材料等自然科学学科", "normalized_name": "科学"},
{"id": "$id:tag:科研", "name": "科研", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "科学研究方法、实验、学术评价、科研工作与科学史。", "normalized_name": "科研"},
{"id": "$id:tag:管理", "name": "管理", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "企业管理、团队、组织、项目、决策与运营效率。", "normalized_name": "管理"},
{"id": "$id:tag:综合", "name": "综合", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "无法稳定归入具体门类的综合性人文内容。", "normalized_name": "综合"},
{"id": "$id:tag:网安", "name": "网安", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "网络安全、密码学、认证授权、漏洞、攻防、隐私与风险控制。", "normalized_name": "网安"},
{"id": "$id:tag:网络", "name": "网络", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "网络协议、路由、域名、代理、通信、接入与网络基础设施。", "normalized_name": "网络"},
{"id": "$id:tag:职业", "name": "职业", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "职业规划、求职招聘、职场经历、劳动关系与专业发展。", "normalized_name": "职业"},
{"id": "$id:tag:航天", "name": "航天", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "天文、航空、航天器、空间探索及轨道科学。", "normalized_name": "航天"},
{"id": "$id:tag:艺术", "name": "艺术", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "绘画、设计、建筑、美术、博物馆及视觉艺术。", "normalized_name": "艺术"},
{"id": "$id:tag:营养", "name": "营养", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "营养、膳食、食品健康、喂养及代谢管理。", "normalized_name": "营养"},
{"id": "$id:tag:营销", "name": "营销", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "品牌、广告、推广、流量、销售及内容商业化。", "normalized_name": "营销"},
{"id": "$id:tag:计算机", "name": "计算机", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "AI、软件、Web 开发、网络安全、云计算及计算机硬件（存储、CPU、GPU 等）", "normalized_name": "计算机"},
{"id": "$id:tag:语言", "name": "语言", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "语言学、文字、语法、方言、翻译及人工语言。", "normalized_name": "语言"},
{"id": "$id:tag:财经", "name": "财经", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "经济、金融、投资、证券、信贷、货币与宏观市场。", "normalized_name": "财经"},
{"id": "$id:tag:运动", "name": "运动", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "健身、游泳、跑步、身体训练与运动健康。", "normalized_name": "运动"},
{"id": "$id:tag:运维", "name": "运维", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "服务器、云平台、容器、部署、监控、备份、迁移与可靠性运维。", "normalized_name": "运维"},
{"id": "$id:tag:音乐", "name": "音乐", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "音乐、歌曲、专辑、乐器、音乐制作与评论。", "normalized_name": "音乐"},
{"id": "$id:tag:饮食", "name": "饮食", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "饮食、烹饪、美食体验及日常餐饮生活。", "normalized_name": "饮食"}
],
"directory.tag_cascades": [
{"id": "$id:cascade:ARTICLE:business/topic-business-career", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 12, "updated_at": "$now", "taxonomy_key": "business/topic-business-career", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:职业", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:business/topic-business-consumption", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 13, "updated_at": "$now", "taxonomy_key": "business/topic-business-consumption", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:消费", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:business/topic-business-entrepreneurship", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 14, "updated_at": "$now", "taxonomy_key": "business/topic-business-entrepreneurship", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:创业", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:business/topic-business-finance", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 15, "updated_at": "$now", "taxonomy_key": "business/topic-business-finance", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:财经", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:business/topic-business-industry", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 16, "updated_at": "$now", "taxonomy_key": "business/topic-business-industry", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:产业", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:business/topic-business-management", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 17, "updated_at": "$now", "taxonomy_key": "business/topic-business-management", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:管理", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:business/topic-business-marketing", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 18, "updated_at": "$now", "taxonomy_key": "business/topic-business-marketing", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:营销", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:business/topic-business-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 19, "updated_at": "$now", "taxonomy_key": "business/topic-business-other", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-ai", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 20, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-ai", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:人工智能", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 21, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-other", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-data", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 22, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-data", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:数据", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-development", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 23, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-development", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:开发", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-hardware", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 24, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-hardware", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:硬件", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-network", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 25, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-network", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:网络", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-operations", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 26, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-operations", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:运维", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-cybersecurity", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 27, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-cybersecurity", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:网安", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-operationsystems", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 28, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-operationsystems", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:操作系统", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:computer/topic-computer-tools", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 29, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-tools", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:工具", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-anime", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 30, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-anime", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:动漫", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-outdoors", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 31, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-outdoors", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:户外", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-collection", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 32, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-collection", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:收藏", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-film", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 33, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-film", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:影视", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-games", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 34, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-games", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:游戏", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-music", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 35, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-music", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:音乐", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-performance", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 36, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-performance", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:演出", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-photography", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 37, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-photography", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:摄影", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-sports", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 38, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-sports", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:体育", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:entertainment/topic-entertainment-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 39, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-other", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:health/topic-health-fitness", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 40, "updated_at": "$now", "taxonomy_key": "health/topic-health-fitness", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:运动", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:health/topic-health-medicine", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 41, "updated_at": "$now", "taxonomy_key": "health/topic-health-medicine", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:医疗", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:health/topic-health-nutrition", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 42, "updated_at": "$now", "taxonomy_key": "health/topic-health-nutrition", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:营养", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:health/topic-health-public-health", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 43, "updated_at": "$now", "taxonomy_key": "health/topic-health-public-health", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:公卫", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:health/topic-health-wellness", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 44, "updated_at": "$now", "taxonomy_key": "health/topic-health-wellness", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:养生", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:health/topic-health-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 45, "updated_at": "$now", "taxonomy_key": "health/topic-health-other", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-art", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 46, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-art", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:艺术", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-culture", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 47, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-culture", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:文化", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-general", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 48, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-general", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:综合", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-history", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 49, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-history", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:历史", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-language", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 50, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-language", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:语言", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-literature", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 51, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-literature", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:文学", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-philosophy", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 52, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-philosophy", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:哲学", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-religion", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 53, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-religion", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:宗教", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-writing", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 54, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-writing", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:写作", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:humanities/topic-humanities-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 55, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-other", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-family", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 56, "updated_at": "$now", "taxonomy_key": "life/topic-life-family", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:家庭", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-food", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 57, "updated_at": "$now", "taxonomy_key": "life/topic-life-food", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:饮食", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-growth", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 58, "updated_at": "$now", "taxonomy_key": "life/topic-life-growth", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:成长", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-home", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 59, "updated_at": "$now", "taxonomy_key": "life/topic-life-home", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:居家", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-pets", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 60, "updated_at": "$now", "taxonomy_key": "life/topic-life-pets", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:宠物", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-psychology", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 61, "updated_at": "$now", "taxonomy_key": "life/topic-life-psychology", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:心理", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-relationships", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 62, "updated_at": "$now", "taxonomy_key": "life/topic-life-relationships", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:情感", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:life/topic-life-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 63, "updated_at": "$now", "taxonomy_key": "life/topic-life-other", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:other/topic-other-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 64, "updated_at": "$now", "taxonomy_key": "other/topic-other-other", "level1_tag_id": "$id:tag:其他", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-aerospace", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 65, "updated_at": "$now", "taxonomy_key": "science/topic-science-aerospace", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:航天", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-biology", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 66, "updated_at": "$now", "taxonomy_key": "science/topic-science-biology", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:生物", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-engineering", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 67, "updated_at": "$now", "taxonomy_key": "science/topic-science-engineering", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:工程", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-environment", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 68, "updated_at": "$now", "taxonomy_key": "science/topic-science-environment", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:环境", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-mathematics", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 69, "updated_at": "$now", "taxonomy_key": "science/topic-science-mathematics", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:数学", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-physics", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 70, "updated_at": "$now", "taxonomy_key": "science/topic-science-physics", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:物理", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-research", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 71, "updated_at": "$now", "taxonomy_key": "science/topic-science-research", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:科研", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:science/topic-science-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 72, "updated_at": "$now", "taxonomy_key": "science/topic-science-other", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:society/topic-society-education", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 73, "updated_at": "$now", "taxonomy_key": "society/topic-society-education", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:教育", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:society/topic-society-ethics", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 74, "updated_at": "$now", "taxonomy_key": "society/topic-society-ethics", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:伦理", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:society/topic-society-law", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 75, "updated_at": "$now", "taxonomy_key": "society/topic-society-law", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:法律", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:society/topic-society-media", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 76, "updated_at": "$now", "taxonomy_key": "society/topic-society-media", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:传媒", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:society/topic-society-politics", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 77, "updated_at": "$now", "taxonomy_key": "society/topic-society-politics", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:政治", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:society/topic-society-public", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 78, "updated_at": "$now", "taxonomy_key": "society/topic-society-public", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:公共", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:society/topic-society-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 79, "updated_at": "$now", "taxonomy_key": "society/topic-society-other", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:travel/topic-travel-guide", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 80, "updated_at": "$now", "taxonomy_key": "travel/topic-travel-guide", "level1_tag_id": "$id:tag:旅行", "level2_tag_id": "$id:tag:攻略", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:travel/topic-travel-journal", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 81, "updated_at": "$now", "taxonomy_key": "travel/topic-travel-journal", "level1_tag_id": "$id:tag:旅行", "level2_tag_id": "$id:tag:游记", "merged_into_id": null},
{"id": "$id:cascade:ARTICLE:travel/topic-travel-other", "scope": "ARTICLE", "created_at": "$now", "is_enabled": true, "sort_order": 82, "updated_at": "$now", "taxonomy_key": "travel/topic-travel-other", "level1_tag_id": "$id:tag:旅行", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-career", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 12, "updated_at": "$now", "taxonomy_key": "business/topic-business-career", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:职业", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-consumption", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 13, "updated_at": "$now", "taxonomy_key": "business/topic-business-consumption", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:消费", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-entrepreneurship", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 14, "updated_at": "$now", "taxonomy_key": "business/topic-business-entrepreneurship", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:创业", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-finance", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 15, "updated_at": "$now", "taxonomy_key": "business/topic-business-finance", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:财经", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-industry", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 16, "updated_at": "$now", "taxonomy_key": "business/topic-business-industry", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:产业", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-management", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 17, "updated_at": "$now", "taxonomy_key": "business/topic-business-management", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:管理", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-marketing", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 18, "updated_at": "$now", "taxonomy_key": "business/topic-business-marketing", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:营销", "merged_into_id": null},
{"id": "$id:cascade:SITE:business/topic-business-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 19, "updated_at": "$now", "taxonomy_key": "business/topic-business-other", "level1_tag_id": "$id:tag:商业", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-ai", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 20, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-ai", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:人工智能", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 21, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-other", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-data", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 22, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-data", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:数据", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-development", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 23, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-development", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:开发", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-hardware", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 24, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-hardware", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:硬件", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-network", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 25, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-network", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:网络", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-operations", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 26, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-operations", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:运维", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-cybersecurity", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 27, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-cybersecurity", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:网安", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-operationsystems", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 28, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-operationsystems", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:操作系统", "merged_into_id": null},
{"id": "$id:cascade:SITE:computer/topic-computer-tools", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 29, "updated_at": "$now", "taxonomy_key": "computer/topic-computer-tools", "level1_tag_id": "$id:tag:计算机", "level2_tag_id": "$id:tag:工具", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-anime", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 30, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-anime", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:动漫", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-outdoors", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 31, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-outdoors", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:户外", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-collection", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 32, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-collection", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:收藏", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-film", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 33, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-film", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:影视", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-games", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 34, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-games", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:游戏", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-music", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 35, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-music", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:音乐", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-performance", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 36, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-performance", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:演出", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-photography", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 37, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-photography", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:摄影", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-sports", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 38, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-sports", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:体育", "merged_into_id": null},
{"id": "$id:cascade:SITE:entertainment/topic-entertainment-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 39, "updated_at": "$now", "taxonomy_key": "entertainment/topic-entertainment-other", "level1_tag_id": "$id:tag:娱乐", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:health/topic-health-fitness", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 40, "updated_at": "$now", "taxonomy_key": "health/topic-health-fitness", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:运动", "merged_into_id": null},
{"id": "$id:cascade:SITE:health/topic-health-medicine", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 41, "updated_at": "$now", "taxonomy_key": "health/topic-health-medicine", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:医疗", "merged_into_id": null},
{"id": "$id:cascade:SITE:health/topic-health-nutrition", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 42, "updated_at": "$now", "taxonomy_key": "health/topic-health-nutrition", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:营养", "merged_into_id": null},
{"id": "$id:cascade:SITE:health/topic-health-public-health", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 43, "updated_at": "$now", "taxonomy_key": "health/topic-health-public-health", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:公卫", "merged_into_id": null},
{"id": "$id:cascade:SITE:health/topic-health-wellness", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 44, "updated_at": "$now", "taxonomy_key": "health/topic-health-wellness", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:养生", "merged_into_id": null},
{"id": "$id:cascade:SITE:health/topic-health-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 45, "updated_at": "$now", "taxonomy_key": "health/topic-health-other", "level1_tag_id": "$id:tag:健康", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-art", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 46, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-art", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:艺术", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-culture", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 47, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-culture", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:文化", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-general", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 48, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-general", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:综合", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-history", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 49, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-history", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:历史", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-language", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 50, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-language", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:语言", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-literature", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 51, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-literature", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:文学", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-philosophy", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 52, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-philosophy", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:哲学", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-religion", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 53, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-religion", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:宗教", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-writing", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 54, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-writing", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:写作", "merged_into_id": null},
{"id": "$id:cascade:SITE:humanities/topic-humanities-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 55, "updated_at": "$now", "taxonomy_key": "humanities/topic-humanities-other", "level1_tag_id": "$id:tag:人文", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-family", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 56, "updated_at": "$now", "taxonomy_key": "life/topic-life-family", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:家庭", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-food", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 57, "updated_at": "$now", "taxonomy_key": "life/topic-life-food", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:饮食", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-growth", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 58, "updated_at": "$now", "taxonomy_key": "life/topic-life-growth", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:成长", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-home", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 59, "updated_at": "$now", "taxonomy_key": "life/topic-life-home", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:居家", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-pets", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 60, "updated_at": "$now", "taxonomy_key": "life/topic-life-pets", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:宠物", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-psychology", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 61, "updated_at": "$now", "taxonomy_key": "life/topic-life-psychology", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:心理", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-relationships", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 62, "updated_at": "$now", "taxonomy_key": "life/topic-life-relationships", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:情感", "merged_into_id": null},
{"id": "$id:cascade:SITE:life/topic-life-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 63, "updated_at": "$now", "taxonomy_key": "life/topic-life-other", "level1_tag_id": "$id:tag:生活", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:other/topic-other-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 64, "updated_at": "$now", "taxonomy_key": "other/topic-other-other", "level1_tag_id": "$id:tag:其他", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-aerospace", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 65, "updated_at": "$now", "taxonomy_key": "science/topic-science-aerospace", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:航天", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-biology", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 66, "updated_at": "$now", "taxonomy_key": "science/topic-science-biology", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:生物", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-engineering", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 67, "updated_at": "$now", "taxonomy_key": "science/topic-science-engineering", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:工程", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-environment", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 68, "updated_at": "$now", "taxonomy_key": "science/topic-science-environment", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:环境", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-mathematics", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 69, "updated_at": "$now", "taxonomy_key": "science/topic-science-mathematics", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:数学", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-physics", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 70, "updated_at": "$now", "taxonomy_key": "science/topic-science-physics", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:物理", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-research", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 71, "updated_at": "$now", "taxonomy_key": "science/topic-science-research", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:科研", "merged_into_id": null},
{"id": "$id:cascade:SITE:science/topic-science-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 72, "updated_at": "$now", "taxonomy_key": "science/topic-science-other", "level1_tag_id": "$id:tag:科学", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:society/topic-society-education", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 73, "updated_at": "$now", "taxonomy_key": "society/topic-society-education", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:教育", "merged_into_id": null},
{"id": "$id:cascade:SITE:society/topic-society-ethics", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 74, "updated_at": "$now", "taxonomy_key": "society/topic-society-ethics", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:伦理", "merged_into_id": null},
{"id": "$id:cascade:SITE:society/topic-society-law", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 75, "updated_at": "$now", "taxonomy_key": "society/topic-society-law", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:法律", "merged_into_id": null},
{"id": "$id:cascade:SITE:society/topic-society-media", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 76, "updated_at": "$now", "taxonomy_key": "society/topic-society-media", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:传媒", "merged_into_id": null},
{"id": "$id:cascade:SITE:society/topic-society-politics", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 77, "updated_at": "$now", "taxonomy_key": "society/topic-society-politics", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:政治", "merged_into_id": null},
{"id": "$id:cascade:SITE:society/topic-society-public", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 78, "updated_at": "$now", "taxonomy_key": "society/topic-society-public", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:公共", "merged_into_id": null},
{"id": "$id:cascade:SITE:society/topic-society-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 79, "updated_at": "$now", "taxonomy_key": "society/topic-society-other", "level1_tag_id": "$id:tag:社会", "level2_tag_id": "$id:tag:其他", "merged_into_id": null},
{"id": "$id:cascade:SITE:travel/topic-travel-guide", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 80, "updated_at": "$now", "taxonomy_key": "travel/topic-travel-guide", "level1_tag_id": "$id:tag:旅行", "level2_tag_id": "$id:tag:攻略", "merged_into_id": null},
{"id": "$id:cascade:SITE:travel/topic-travel-journal", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 81, "updated_at": "$now", "taxonomy_key": "travel/topic-travel-journal", "level1_tag_id": "$id:tag:旅行", "level2_tag_id": "$id:tag:游记", "merged_into_id": null},
{"id": "$id:cascade:SITE:travel/topic-travel-other", "scope": "SITE", "created_at": "$now", "is_enabled": true, "sort_order": 82, "updated_at": "$now", "taxonomy_key": "travel/topic-travel-other", "level1_tag_id": "$id:tag:旅行", "level2_tag_id": "$id:tag:其他", "merged_into_id": null}
],
"directory.software_components": [
{"id": "$id:component:其他", "name": "其他", "created_at": "$now", "is_enabled": true, "updated_at": "$now", "description": "站点程序未列入目录时使用此选项。", "homepage_url": null, "is_open_source": false, "repository_url": null, "normalized_name": "其他"}
],
"directory.tag_identity_aliases": [
{"tag_id":"$id:tag:商业","alias_id":"$id:tag:商业","snapshot":{"id":"$id:tag:商业","name":"商业"},"alias_kind":"TAG","system_key":"business","archived_at":"$now"},
{"tag_id":"$id:tag:计算机","alias_id":"$id:tag:计算机","snapshot":{"id":"$id:tag:计算机","name":"计算机"},"alias_kind":"TAG","system_key":"computer","archived_at":"$now"},
{"tag_id":"$id:tag:娱乐","alias_id":"$id:tag:娱乐","snapshot":{"id":"$id:tag:娱乐","name":"娱乐"},"alias_kind":"TAG","system_key":"entertainment","archived_at":"$now"},
{"tag_id":"$id:tag:健康","alias_id":"$id:tag:健康","snapshot":{"id":"$id:tag:健康","name":"健康"},"alias_kind":"TAG","system_key":"health","archived_at":"$now"},
{"tag_id":"$id:tag:人文","alias_id":"$id:tag:人文","snapshot":{"id":"$id:tag:人文","name":"人文"},"alias_kind":"TAG","system_key":"humanities","archived_at":"$now"},
{"tag_id":"$id:tag:生活","alias_id":"$id:tag:生活","snapshot":{"id":"$id:tag:生活","name":"生活"},"alias_kind":"TAG","system_key":"life","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:tag:其他","snapshot":{"id":"$id:tag:其他","name":"其他"},"alias_kind":"TAG","system_key":"other","archived_at":"$now"},
{"tag_id":"$id:tag:科学","alias_id":"$id:tag:科学","snapshot":{"id":"$id:tag:科学","name":"科学"},"alias_kind":"TAG","system_key":"science","archived_at":"$now"},
{"tag_id":"$id:tag:社会","alias_id":"$id:tag:社会","snapshot":{"id":"$id:tag:社会","name":"社会"},"alias_kind":"TAG","system_key":"society","archived_at":"$now"},
{"tag_id":"$id:tag:职业","alias_id":"$id:tag:职业","snapshot":{"id":"$id:tag:职业","name":"职业"},"alias_kind":"TAG","system_key":"topic-business-career","archived_at":"$now"},
{"tag_id":"$id:tag:消费","alias_id":"$id:tag:消费","snapshot":{"id":"$id:tag:消费","name":"消费"},"alias_kind":"TAG","system_key":"topic-business-consumption","archived_at":"$now"},
{"tag_id":"$id:tag:创业","alias_id":"$id:tag:创业","snapshot":{"id":"$id:tag:创业","name":"创业"},"alias_kind":"TAG","system_key":"topic-business-entrepreneurship","archived_at":"$now"},
{"tag_id":"$id:tag:财经","alias_id":"$id:tag:财经","snapshot":{"id":"$id:tag:财经","name":"财经"},"alias_kind":"TAG","system_key":"topic-business-finance","archived_at":"$now"},
{"tag_id":"$id:tag:产业","alias_id":"$id:tag:产业","snapshot":{"id":"$id:tag:产业","name":"产业"},"alias_kind":"TAG","system_key":"topic-business-industry","archived_at":"$now"},
{"tag_id":"$id:tag:管理","alias_id":"$id:tag:管理","snapshot":{"id":"$id:tag:管理","name":"管理"},"alias_kind":"TAG","system_key":"topic-business-management","archived_at":"$now"},
{"tag_id":"$id:tag:营销","alias_id":"$id:tag:营销","snapshot":{"id":"$id:tag:营销","name":"营销"},"alias_kind":"TAG","system_key":"topic-business-marketing","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-business-other","snapshot":{"id":"$id:alias:TAG:topic-business-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-business-other","archived_at":"$now"},
{"tag_id":"$id:tag:人工智能","alias_id":"$id:tag:人工智能","snapshot":{"id":"$id:tag:人工智能","name":"人工智能"},"alias_kind":"TAG","system_key":"topic-computer-ai","archived_at":"$now"},
{"tag_id":"$id:tag:网安","alias_id":"$id:tag:网安","snapshot":{"id":"$id:tag:网安","name":"网安"},"alias_kind":"TAG","system_key":"topic-computer-cybersecurity","archived_at":"$now"},
{"tag_id":"$id:tag:数据","alias_id":"$id:tag:数据","snapshot":{"id":"$id:tag:数据","name":"数据"},"alias_kind":"TAG","system_key":"topic-computer-data","archived_at":"$now"},
{"tag_id":"$id:tag:开发","alias_id":"$id:tag:开发","snapshot":{"id":"$id:tag:开发","name":"开发"},"alias_kind":"TAG","system_key":"topic-computer-development","archived_at":"$now"},
{"tag_id":"$id:tag:硬件","alias_id":"$id:tag:硬件","snapshot":{"id":"$id:tag:硬件","name":"硬件"},"alias_kind":"TAG","system_key":"topic-computer-hardware","archived_at":"$now"},
{"tag_id":"$id:tag:网络","alias_id":"$id:tag:网络","snapshot":{"id":"$id:tag:网络","name":"网络"},"alias_kind":"TAG","system_key":"topic-computer-network","archived_at":"$now"},
{"tag_id":"$id:tag:运维","alias_id":"$id:tag:运维","snapshot":{"id":"$id:tag:运维","name":"运维"},"alias_kind":"TAG","system_key":"topic-computer-operations","archived_at":"$now"},
{"tag_id":"$id:tag:操作系统","alias_id":"$id:tag:操作系统","snapshot":{"id":"$id:tag:操作系统","name":"操作系统"},"alias_kind":"TAG","system_key":"topic-computer-operationsystems","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-computer-other","snapshot":{"id":"$id:alias:TAG:topic-computer-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-computer-other","archived_at":"$now"},
{"tag_id":"$id:tag:工具","alias_id":"$id:tag:工具","snapshot":{"id":"$id:tag:工具","name":"工具"},"alias_kind":"TAG","system_key":"topic-computer-tools","archived_at":"$now"},
{"tag_id":"$id:tag:动漫","alias_id":"$id:tag:动漫","snapshot":{"id":"$id:tag:动漫","name":"动漫"},"alias_kind":"TAG","system_key":"topic-entertainment-anime","archived_at":"$now"},
{"tag_id":"$id:tag:收藏","alias_id":"$id:tag:收藏","snapshot":{"id":"$id:tag:收藏","name":"收藏"},"alias_kind":"TAG","system_key":"topic-entertainment-collection","archived_at":"$now"},
{"tag_id":"$id:tag:影视","alias_id":"$id:tag:影视","snapshot":{"id":"$id:tag:影视","name":"影视"},"alias_kind":"TAG","system_key":"topic-entertainment-film","archived_at":"$now"},
{"tag_id":"$id:tag:游戏","alias_id":"$id:tag:游戏","snapshot":{"id":"$id:tag:游戏","name":"游戏"},"alias_kind":"TAG","system_key":"topic-entertainment-games","archived_at":"$now"},
{"tag_id":"$id:tag:音乐","alias_id":"$id:tag:音乐","snapshot":{"id":"$id:tag:音乐","name":"音乐"},"alias_kind":"TAG","system_key":"topic-entertainment-music","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-entertainment-other","snapshot":{"id":"$id:alias:TAG:topic-entertainment-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-entertainment-other","archived_at":"$now"},
{"tag_id":"$id:tag:户外","alias_id":"$id:tag:户外","snapshot":{"id":"$id:tag:户外","name":"户外"},"alias_kind":"TAG","system_key":"topic-entertainment-outdoors","archived_at":"$now"},
{"tag_id":"$id:tag:演出","alias_id":"$id:tag:演出","snapshot":{"id":"$id:tag:演出","name":"演出"},"alias_kind":"TAG","system_key":"topic-entertainment-performance","archived_at":"$now"},
{"tag_id":"$id:tag:摄影","alias_id":"$id:tag:摄影","snapshot":{"id":"$id:tag:摄影","name":"摄影"},"alias_kind":"TAG","system_key":"topic-entertainment-photography","archived_at":"$now"},
{"tag_id":"$id:tag:体育","alias_id":"$id:tag:体育","snapshot":{"id":"$id:tag:体育","name":"体育"},"alias_kind":"TAG","system_key":"topic-entertainment-sports","archived_at":"$now"},
{"tag_id":"$id:tag:运动","alias_id":"$id:tag:运动","snapshot":{"id":"$id:tag:运动","name":"运动"},"alias_kind":"TAG","system_key":"topic-health-fitness","archived_at":"$now"},
{"tag_id":"$id:tag:医疗","alias_id":"$id:tag:医疗","snapshot":{"id":"$id:tag:医疗","name":"医疗"},"alias_kind":"TAG","system_key":"topic-health-medicine","archived_at":"$now"},
{"tag_id":"$id:tag:营养","alias_id":"$id:tag:营养","snapshot":{"id":"$id:tag:营养","name":"营养"},"alias_kind":"TAG","system_key":"topic-health-nutrition","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-health-other","snapshot":{"id":"$id:alias:TAG:topic-health-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-health-other","archived_at":"$now"},
{"tag_id":"$id:tag:公卫","alias_id":"$id:tag:公卫","snapshot":{"id":"$id:tag:公卫","name":"公卫"},"alias_kind":"TAG","system_key":"topic-health-public-health","archived_at":"$now"},
{"tag_id":"$id:tag:养生","alias_id":"$id:tag:养生","snapshot":{"id":"$id:tag:养生","name":"养生"},"alias_kind":"TAG","system_key":"topic-health-wellness","archived_at":"$now"},
{"tag_id":"$id:tag:艺术","alias_id":"$id:tag:艺术","snapshot":{"id":"$id:tag:艺术","name":"艺术"},"alias_kind":"TAG","system_key":"topic-humanities-art","archived_at":"$now"},
{"tag_id":"$id:tag:文化","alias_id":"$id:tag:文化","snapshot":{"id":"$id:tag:文化","name":"文化"},"alias_kind":"TAG","system_key":"topic-humanities-culture","archived_at":"$now"},
{"tag_id":"$id:tag:综合","alias_id":"$id:tag:综合","snapshot":{"id":"$id:tag:综合","name":"综合"},"alias_kind":"TAG","system_key":"topic-humanities-general","archived_at":"$now"},
{"tag_id":"$id:tag:历史","alias_id":"$id:tag:历史","snapshot":{"id":"$id:tag:历史","name":"历史"},"alias_kind":"TAG","system_key":"topic-humanities-history","archived_at":"$now"},
{"tag_id":"$id:tag:语言","alias_id":"$id:tag:语言","snapshot":{"id":"$id:tag:语言","name":"语言"},"alias_kind":"TAG","system_key":"topic-humanities-language","archived_at":"$now"},
{"tag_id":"$id:tag:文学","alias_id":"$id:tag:文学","snapshot":{"id":"$id:tag:文学","name":"文学"},"alias_kind":"TAG","system_key":"topic-humanities-literature","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-humanities-other","snapshot":{"id":"$id:alias:TAG:topic-humanities-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-humanities-other","archived_at":"$now"},
{"tag_id":"$id:tag:哲学","alias_id":"$id:tag:哲学","snapshot":{"id":"$id:tag:哲学","name":"哲学"},"alias_kind":"TAG","system_key":"topic-humanities-philosophy","archived_at":"$now"},
{"tag_id":"$id:tag:宗教","alias_id":"$id:tag:宗教","snapshot":{"id":"$id:tag:宗教","name":"宗教"},"alias_kind":"TAG","system_key":"topic-humanities-religion","archived_at":"$now"},
{"tag_id":"$id:tag:写作","alias_id":"$id:tag:写作","snapshot":{"id":"$id:tag:写作","name":"写作"},"alias_kind":"TAG","system_key":"topic-humanities-writing","archived_at":"$now"},
{"tag_id":"$id:tag:家庭","alias_id":"$id:tag:家庭","snapshot":{"id":"$id:tag:家庭","name":"家庭"},"alias_kind":"TAG","system_key":"topic-life-family","archived_at":"$now"},
{"tag_id":"$id:tag:饮食","alias_id":"$id:tag:饮食","snapshot":{"id":"$id:tag:饮食","name":"饮食"},"alias_kind":"TAG","system_key":"topic-life-food","archived_at":"$now"},
{"tag_id":"$id:tag:成长","alias_id":"$id:tag:成长","snapshot":{"id":"$id:tag:成长","name":"成长"},"alias_kind":"TAG","system_key":"topic-life-growth","archived_at":"$now"},
{"tag_id":"$id:tag:居家","alias_id":"$id:tag:居家","snapshot":{"id":"$id:tag:居家","name":"居家"},"alias_kind":"TAG","system_key":"topic-life-home","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-life-other","snapshot":{"id":"$id:alias:TAG:topic-life-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-life-other","archived_at":"$now"},
{"tag_id":"$id:tag:宠物","alias_id":"$id:tag:宠物","snapshot":{"id":"$id:tag:宠物","name":"宠物"},"alias_kind":"TAG","system_key":"topic-life-pets","archived_at":"$now"},
{"tag_id":"$id:tag:心理","alias_id":"$id:tag:心理","snapshot":{"id":"$id:tag:心理","name":"心理"},"alias_kind":"TAG","system_key":"topic-life-psychology","archived_at":"$now"},
{"tag_id":"$id:tag:情感","alias_id":"$id:tag:情感","snapshot":{"id":"$id:tag:情感","name":"情感"},"alias_kind":"TAG","system_key":"topic-life-relationships","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-other-other","snapshot":{"id":"$id:alias:TAG:topic-other-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-other-other","archived_at":"$now"},
{"tag_id":"$id:tag:航天","alias_id":"$id:tag:航天","snapshot":{"id":"$id:tag:航天","name":"航天"},"alias_kind":"TAG","system_key":"topic-science-aerospace","archived_at":"$now"},
{"tag_id":"$id:tag:生物","alias_id":"$id:tag:生物","snapshot":{"id":"$id:tag:生物","name":"生物"},"alias_kind":"TAG","system_key":"topic-science-biology","archived_at":"$now"},
{"tag_id":"$id:tag:工程","alias_id":"$id:tag:工程","snapshot":{"id":"$id:tag:工程","name":"工程"},"alias_kind":"TAG","system_key":"topic-science-engineering","archived_at":"$now"},
{"tag_id":"$id:tag:环境","alias_id":"$id:tag:环境","snapshot":{"id":"$id:tag:环境","name":"环境"},"alias_kind":"TAG","system_key":"topic-science-environment","archived_at":"$now"},
{"tag_id":"$id:tag:数学","alias_id":"$id:tag:数学","snapshot":{"id":"$id:tag:数学","name":"数学"},"alias_kind":"TAG","system_key":"topic-science-mathematics","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-science-other","snapshot":{"id":"$id:alias:TAG:topic-science-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-science-other","archived_at":"$now"},
{"tag_id":"$id:tag:物理","alias_id":"$id:tag:物理","snapshot":{"id":"$id:tag:物理","name":"物理"},"alias_kind":"TAG","system_key":"topic-science-physics","archived_at":"$now"},
{"tag_id":"$id:tag:科研","alias_id":"$id:tag:科研","snapshot":{"id":"$id:tag:科研","name":"科研"},"alias_kind":"TAG","system_key":"topic-science-research","archived_at":"$now"},
{"tag_id":"$id:tag:教育","alias_id":"$id:tag:教育","snapshot":{"id":"$id:tag:教育","name":"教育"},"alias_kind":"TAG","system_key":"topic-society-education","archived_at":"$now"},
{"tag_id":"$id:tag:伦理","alias_id":"$id:tag:伦理","snapshot":{"id":"$id:tag:伦理","name":"伦理"},"alias_kind":"TAG","system_key":"topic-society-ethics","archived_at":"$now"},
{"tag_id":"$id:tag:法律","alias_id":"$id:tag:法律","snapshot":{"id":"$id:tag:法律","name":"法律"},"alias_kind":"TAG","system_key":"topic-society-law","archived_at":"$now"},
{"tag_id":"$id:tag:传媒","alias_id":"$id:tag:传媒","snapshot":{"id":"$id:tag:传媒","name":"传媒"},"alias_kind":"TAG","system_key":"topic-society-media","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-society-other","snapshot":{"id":"$id:alias:TAG:topic-society-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-society-other","archived_at":"$now"},
{"tag_id":"$id:tag:政治","alias_id":"$id:tag:政治","snapshot":{"id":"$id:tag:政治","name":"政治"},"alias_kind":"TAG","system_key":"topic-society-politics","archived_at":"$now"},
{"tag_id":"$id:tag:公共","alias_id":"$id:tag:公共","snapshot":{"id":"$id:tag:公共","name":"公共"},"alias_kind":"TAG","system_key":"topic-society-public","archived_at":"$now"},
{"tag_id":"$id:tag:攻略","alias_id":"$id:tag:攻略","snapshot":{"id":"$id:tag:攻略","name":"攻略"},"alias_kind":"TAG","system_key":"topic-travel-guide","archived_at":"$now"},
{"tag_id":"$id:tag:游记","alias_id":"$id:tag:游记","snapshot":{"id":"$id:tag:游记","name":"游记"},"alias_kind":"TAG","system_key":"topic-travel-journal","archived_at":"$now"},
{"tag_id":"$id:tag:其他","alias_id":"$id:alias:TAG:topic-travel-other","snapshot":{"id":"$id:alias:TAG:topic-travel-other","name":"其他"},"alias_kind":"TAG","system_key":"topic-travel-other","archived_at":"$now"},
{"tag_id":"$id:tag:旅行","alias_id":"$id:tag:旅行","snapshot":{"id":"$id:tag:旅行","name":"旅行"},"alias_kind":"TAG","system_key":"travel","archived_at":"$now"}
]
}$data$::jsonb;
    seed_text text := data::text;
    identity_key text;
BEGIN

    FOREACH identity_key IN ARRAY ARRAY['$id:alias:TAG:topic-business-other','$id:alias:TAG:topic-computer-other','$id:alias:TAG:topic-entertainment-other','$id:alias:TAG:topic-health-other','$id:alias:TAG:topic-humanities-other','$id:alias:TAG:topic-life-other','$id:alias:TAG:topic-other-other','$id:alias:TAG:topic-science-other','$id:alias:TAG:topic-society-other','$id:alias:TAG:topic-travel-other','$id:cascade:ARTICLE:business/topic-business-career','$id:cascade:ARTICLE:business/topic-business-consumption','$id:cascade:ARTICLE:business/topic-business-entrepreneurship','$id:cascade:ARTICLE:business/topic-business-finance','$id:cascade:ARTICLE:business/topic-business-industry','$id:cascade:ARTICLE:business/topic-business-management','$id:cascade:ARTICLE:business/topic-business-marketing','$id:cascade:ARTICLE:business/topic-business-other','$id:cascade:ARTICLE:computer/topic-computer-ai','$id:cascade:ARTICLE:computer/topic-computer-cybersecurity','$id:cascade:ARTICLE:computer/topic-computer-data','$id:cascade:ARTICLE:computer/topic-computer-development','$id:cascade:ARTICLE:computer/topic-computer-hardware','$id:cascade:ARTICLE:computer/topic-computer-network','$id:cascade:ARTICLE:computer/topic-computer-operations','$id:cascade:ARTICLE:computer/topic-computer-operationsystems','$id:cascade:ARTICLE:computer/topic-computer-other','$id:cascade:ARTICLE:computer/topic-computer-tools','$id:cascade:ARTICLE:entertainment/topic-entertainment-anime','$id:cascade:ARTICLE:entertainment/topic-entertainment-collection','$id:cascade:ARTICLE:entertainment/topic-entertainment-film','$id:cascade:ARTICLE:entertainment/topic-entertainment-games','$id:cascade:ARTICLE:entertainment/topic-entertainment-music','$id:cascade:ARTICLE:entertainment/topic-entertainment-other','$id:cascade:ARTICLE:entertainment/topic-entertainment-outdoors','$id:cascade:ARTICLE:entertainment/topic-entertainment-performance','$id:cascade:ARTICLE:entertainment/topic-entertainment-photography','$id:cascade:ARTICLE:entertainment/topic-entertainment-sports','$id:cascade:ARTICLE:health/topic-health-fitness','$id:cascade:ARTICLE:health/topic-health-medicine','$id:cascade:ARTICLE:health/topic-health-nutrition','$id:cascade:ARTICLE:health/topic-health-other','$id:cascade:ARTICLE:health/topic-health-public-health','$id:cascade:ARTICLE:health/topic-health-wellness','$id:cascade:ARTICLE:humanities/topic-humanities-art','$id:cascade:ARTICLE:humanities/topic-humanities-culture','$id:cascade:ARTICLE:humanities/topic-humanities-general','$id:cascade:ARTICLE:humanities/topic-humanities-history','$id:cascade:ARTICLE:humanities/topic-humanities-language','$id:cascade:ARTICLE:humanities/topic-humanities-literature','$id:cascade:ARTICLE:humanities/topic-humanities-other','$id:cascade:ARTICLE:humanities/topic-humanities-philosophy','$id:cascade:ARTICLE:humanities/topic-humanities-religion','$id:cascade:ARTICLE:humanities/topic-humanities-writing','$id:cascade:ARTICLE:life/topic-life-family','$id:cascade:ARTICLE:life/topic-life-food','$id:cascade:ARTICLE:life/topic-life-growth','$id:cascade:ARTICLE:life/topic-life-home','$id:cascade:ARTICLE:life/topic-life-other','$id:cascade:ARTICLE:life/topic-life-pets','$id:cascade:ARTICLE:life/topic-life-psychology','$id:cascade:ARTICLE:life/topic-life-relationships','$id:cascade:ARTICLE:other/topic-other-other','$id:cascade:ARTICLE:science/topic-science-aerospace','$id:cascade:ARTICLE:science/topic-science-biology','$id:cascade:ARTICLE:science/topic-science-engineering','$id:cascade:ARTICLE:science/topic-science-environment','$id:cascade:ARTICLE:science/topic-science-mathematics','$id:cascade:ARTICLE:science/topic-science-other','$id:cascade:ARTICLE:science/topic-science-physics','$id:cascade:ARTICLE:science/topic-science-research','$id:cascade:ARTICLE:society/topic-society-education','$id:cascade:ARTICLE:society/topic-society-ethics','$id:cascade:ARTICLE:society/topic-society-law','$id:cascade:ARTICLE:society/topic-society-media','$id:cascade:ARTICLE:society/topic-society-other','$id:cascade:ARTICLE:society/topic-society-politics','$id:cascade:ARTICLE:society/topic-society-public','$id:cascade:ARTICLE:travel/topic-travel-guide','$id:cascade:ARTICLE:travel/topic-travel-journal','$id:cascade:ARTICLE:travel/topic-travel-other','$id:cascade:SITE:business/topic-business-career','$id:cascade:SITE:business/topic-business-consumption','$id:cascade:SITE:business/topic-business-entrepreneurship','$id:cascade:SITE:business/topic-business-finance','$id:cascade:SITE:business/topic-business-industry','$id:cascade:SITE:business/topic-business-management','$id:cascade:SITE:business/topic-business-marketing','$id:cascade:SITE:business/topic-business-other','$id:cascade:SITE:computer/topic-computer-ai','$id:cascade:SITE:computer/topic-computer-cybersecurity','$id:cascade:SITE:computer/topic-computer-data','$id:cascade:SITE:computer/topic-computer-development','$id:cascade:SITE:computer/topic-computer-hardware','$id:cascade:SITE:computer/topic-computer-network','$id:cascade:SITE:computer/topic-computer-operations','$id:cascade:SITE:computer/topic-computer-operationsystems','$id:cascade:SITE:computer/topic-computer-other','$id:cascade:SITE:computer/topic-computer-tools','$id:cascade:SITE:entertainment/topic-entertainment-anime','$id:cascade:SITE:entertainment/topic-entertainment-collection','$id:cascade:SITE:entertainment/topic-entertainment-film','$id:cascade:SITE:entertainment/topic-entertainment-games','$id:cascade:SITE:entertainment/topic-entertainment-music','$id:cascade:SITE:entertainment/topic-entertainment-other','$id:cascade:SITE:entertainment/topic-entertainment-outdoors','$id:cascade:SITE:entertainment/topic-entertainment-performance','$id:cascade:SITE:entertainment/topic-entertainment-photography','$id:cascade:SITE:entertainment/topic-entertainment-sports','$id:cascade:SITE:health/topic-health-fitness','$id:cascade:SITE:health/topic-health-medicine','$id:cascade:SITE:health/topic-health-nutrition','$id:cascade:SITE:health/topic-health-other','$id:cascade:SITE:health/topic-health-public-health','$id:cascade:SITE:health/topic-health-wellness','$id:cascade:SITE:humanities/topic-humanities-art','$id:cascade:SITE:humanities/topic-humanities-culture','$id:cascade:SITE:humanities/topic-humanities-general','$id:cascade:SITE:humanities/topic-humanities-history','$id:cascade:SITE:humanities/topic-humanities-language','$id:cascade:SITE:humanities/topic-humanities-literature','$id:cascade:SITE:humanities/topic-humanities-other','$id:cascade:SITE:humanities/topic-humanities-philosophy','$id:cascade:SITE:humanities/topic-humanities-religion','$id:cascade:SITE:humanities/topic-humanities-writing','$id:cascade:SITE:life/topic-life-family','$id:cascade:SITE:life/topic-life-food','$id:cascade:SITE:life/topic-life-growth','$id:cascade:SITE:life/topic-life-home','$id:cascade:SITE:life/topic-life-other','$id:cascade:SITE:life/topic-life-pets','$id:cascade:SITE:life/topic-life-psychology','$id:cascade:SITE:life/topic-life-relationships','$id:cascade:SITE:other/topic-other-other','$id:cascade:SITE:science/topic-science-aerospace','$id:cascade:SITE:science/topic-science-biology','$id:cascade:SITE:science/topic-science-engineering','$id:cascade:SITE:science/topic-science-environment','$id:cascade:SITE:science/topic-science-mathematics','$id:cascade:SITE:science/topic-science-other','$id:cascade:SITE:science/topic-science-physics','$id:cascade:SITE:science/topic-science-research','$id:cascade:SITE:society/topic-society-education','$id:cascade:SITE:society/topic-society-ethics','$id:cascade:SITE:society/topic-society-law','$id:cascade:SITE:society/topic-society-media','$id:cascade:SITE:society/topic-society-other','$id:cascade:SITE:society/topic-society-politics','$id:cascade:SITE:society/topic-society-public','$id:cascade:SITE:travel/topic-travel-guide','$id:cascade:SITE:travel/topic-travel-journal','$id:cascade:SITE:travel/topic-travel-other','$id:component:其他','$id:tag:产业','$id:tag:人工智能','$id:tag:人文','$id:tag:传媒','$id:tag:伦理','$id:tag:体育','$id:tag:健康','$id:tag:公共','$id:tag:公卫','$id:tag:其他','$id:tag:养生','$id:tag:写作','$id:tag:创业','$id:tag:动漫','$id:tag:医疗','$id:tag:历史','$id:tag:哲学','$id:tag:商业','$id:tag:娱乐','$id:tag:宗教','$id:tag:宠物','$id:tag:家庭','$id:tag:居家','$id:tag:工具','$id:tag:工程','$id:tag:开发','$id:tag:影视','$id:tag:心理','$id:tag:情感','$id:tag:成长','$id:tag:户外','$id:tag:摄影','$id:tag:操作系统','$id:tag:收藏','$id:tag:攻略','$id:tag:政治','$id:tag:教育','$id:tag:数学','$id:tag:数据','$id:tag:文化','$id:tag:文学','$id:tag:旅行','$id:tag:法律','$id:tag:消费','$id:tag:游戏','$id:tag:游记','$id:tag:演出','$id:tag:物理','$id:tag:环境','$id:tag:生活','$id:tag:生物','$id:tag:硬件','$id:tag:社会','$id:tag:科学','$id:tag:科研','$id:tag:管理','$id:tag:综合','$id:tag:网安','$id:tag:网络','$id:tag:职业','$id:tag:航天','$id:tag:艺术','$id:tag:营养','$id:tag:营销','$id:tag:计算机','$id:tag:语言','$id:tag:财经','$id:tag:运动','$id:tag:运维','$id:tag:音乐','$id:tag:饮食'] LOOP

        seed_text := replace(seed_text, to_jsonb(identity_key)::text, to_jsonb(uuidv7()::text)::text);
    END LOOP;
    data := replace(seed_text, '"$now"', to_jsonb(now())::text)::jsonb;

    INSERT INTO directory.tags (id, name, created_at, is_enabled, updated_at, description)
    SELECT id, name, created_at, is_enabled, updated_at, description FROM jsonb_populate_recordset(NULL::directory.tags, data->'directory.tags');

    INSERT INTO directory.tag_cascades (id, scope, created_at, is_enabled, sort_order, updated_at, taxonomy_key, level1_tag_id, level2_tag_id, merged_into_id)
    SELECT id, scope, created_at, is_enabled, sort_order, updated_at, taxonomy_key, level1_tag_id, level2_tag_id, merged_into_id FROM jsonb_populate_recordset(NULL::directory.tag_cascades, data->'directory.tag_cascades');

    INSERT INTO directory.tag_identity_aliases (tag_id, alias_id, snapshot, alias_kind, system_key, archived_at)
    SELECT tag_id, alias_id, snapshot, alias_kind, system_key, archived_at FROM jsonb_populate_recordset(NULL::directory.tag_identity_aliases, data->'directory.tag_identity_aliases');

    INSERT INTO directory.software_components (id, name, created_at, is_enabled, updated_at, description, homepage_url, is_open_source, repository_url, normalized_name)
    SELECT id, name, created_at, is_enabled, updated_at, description, homepage_url, is_open_source, repository_url, normalized_name FROM jsonb_populate_recordset(NULL::directory.software_components, data->'directory.software_components');

END;
$seed$;

-- +goose StatementEnd

CREATE TABLE runtime_cache.taxonomy_revision (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton), -- Restricts taxonomy invalidation state to one row.
    revision bigint NOT NULL DEFAULT 0 -- Transactional generation; rolled-back writes do not invalidate catalogs.
);
COMMENT ON TABLE runtime_cache.taxonomy_revision IS 'Committed generation of all inputs to the management taxonomy catalog and revision.';
COMMENT ON COLUMN runtime_cache.taxonomy_revision.singleton IS 'Restricts taxonomy invalidation state to one row.';
COMMENT ON COLUMN runtime_cache.taxonomy_revision.revision IS 'Transactional generation; rolled-back writes do not invalidate catalogs.';
INSERT INTO runtime_cache.taxonomy_revision DEFAULT VALUES;
GRANT SELECT ON runtime_cache.taxonomy_revision TO api_runtime;

-- +goose StatementBegin
CREATE FUNCTION runtime_cache.invalidate_taxonomy() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    UPDATE runtime_cache.taxonomy_revision SET revision = revision + 1 WHERE singleton;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
COMMENT ON FUNCTION runtime_cache.invalidate_taxonomy() IS 'Invalidates taxonomy catalogs in the same transaction as their source writes.';
REVOKE ALL ON FUNCTION runtime_cache.invalidate_taxonomy() FROM PUBLIC;
CREATE TRIGGER invalidate_taxonomy AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON directory.tags
FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_taxonomy();
COMMENT ON TRIGGER invalidate_taxonomy ON directory.tags IS 'Invalidates taxonomy usage and revision after source changes.';
CREATE TRIGGER invalidate_taxonomy AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON directory.tag_cascades
FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_taxonomy();
COMMENT ON TRIGGER invalidate_taxonomy ON directory.tag_cascades IS 'Invalidates taxonomy usage and revision after source changes.';
CREATE TRIGGER invalidate_taxonomy AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON directory.tag_identity_aliases
FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_taxonomy();
COMMENT ON TRIGGER invalidate_taxonomy ON directory.tag_identity_aliases IS 'Invalidates taxonomy usage and revision after source changes.';
CREATE TRIGGER invalidate_taxonomy AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON directory.sites
FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_taxonomy();
COMMENT ON TRIGGER invalidate_taxonomy ON directory.sites IS 'Invalidates taxonomy usage and revision after source changes.';
CREATE TRIGGER invalidate_taxonomy AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON directory.site_tags
FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_taxonomy();
COMMENT ON TRIGGER invalidate_taxonomy ON directory.site_tags IS 'Invalidates taxonomy usage and revision after source changes.';
CREATE TRIGGER invalidate_taxonomy AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON content.articles
FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_taxonomy();
COMMENT ON TRIGGER invalidate_taxonomy ON content.articles IS 'Invalidates taxonomy usage and revision after source changes.';
CREATE TRIGGER invalidate_taxonomy AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON content.article_tags
FOR EACH STATEMENT EXECUTE FUNCTION runtime_cache.invalidate_taxonomy();
COMMENT ON TRIGGER invalidate_taxonomy ON content.article_tags IS 'Invalidates taxonomy usage and revision after source changes.';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'The consolidated baseline is forward-only; restore the previous database backup.'; END $$;
-- +goose StatementEnd
