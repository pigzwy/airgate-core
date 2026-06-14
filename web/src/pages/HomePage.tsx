import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { Button, Card, Link as HeroLink } from '@heroui/react';
import { useSiteSettings, defaultLogoUrl } from '../app/providers/SiteSettingsProvider';
import { useTheme } from '../app/providers/ThemeProvider';
import { getToken } from '../shared/api/client';
import { effectiveDocUrl } from '../shared/utils/docUrl';
import { useStatusPageEnabled } from '../shared/hooks/useStatusPageEnabled';
import {
  Zap, Shield, Globe, ArrowRight, Sun, Moon, Code, BarChart3, KeyRound, Layers, Activity, Copy, Check,
} from 'lucide-react';

export default function HomePage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const site = useSiteSettings();
  const { theme, toggleTheme } = useTheme();
  const showStatusEntry = useStatusPageEnabled();

  const isLoggedIn = !!getToken();
  // 文档链接 fallback：管理员未填外部 doc_url 时回退到内置 /docs（详见 docUrl.ts）
  const docs = effectiveDocUrl(site.doc_url);

  const features = [
    { icon: <Zap className="w-6 h-6" />, titleKey: 'home.feature_gateway', descKey: 'home.feature_gateway_desc' },
    { icon: <Shield className="w-6 h-6" />, titleKey: 'home.feature_security', descKey: 'home.feature_security_desc' },
    { icon: <Layers className="w-6 h-6" />, titleKey: 'home.feature_plugins', descKey: 'home.feature_plugins_desc' },
    { icon: <BarChart3 className="w-6 h-6" />, titleKey: 'home.feature_analytics', descKey: 'home.feature_analytics_desc' },
    { icon: <KeyRound className="w-6 h-6" />, titleKey: 'home.feature_keys', descKey: 'home.feature_keys_desc' },
    { icon: <Globe className="w-6 h-6" />, titleKey: 'home.feature_multi_platform', descKey: 'home.feature_multi_platform_desc' },
  ];

  // Endpoint 面板：从配置的 api_base_url（缺省回退到当前站点 origin）派生三协议入口。
  // AirGate 的实际可用协议取决于装了哪些网关插件，这里只展示「兼容协议入口」，不伪造在线状态。
  const baseUrl = useMemo(() => {
    const raw = site.api_base_url || (typeof window !== 'undefined' ? window.location.origin : '');
    return raw.replace(/\/+$/, '');
  }, [site.api_base_url]);

  const endpoints = useMemo(
    () => [
      { protocol: 'OpenAI', url: `${baseUrl}/v1` },
      { protocol: 'Anthropic', url: baseUrl },
      { protocol: 'Gemini', url: `${baseUrl}/v1beta` },
    ],
    [baseUrl],
  );

  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const copy = (key: string, text: string) => {
    const done = () => {
      setCopiedKey(key);
      setTimeout(() => setCopiedKey((k) => (k === key ? null : k)), 1500);
    };
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(text).then(done).catch(() => {});
    }
  };

  const quickStart = `export OPENAI_BASE_URL="${baseUrl}/v1"\nexport OPENAI_API_KEY="sk-..."`;

  return (
    <div className="min-h-[100dvh] bg-background text-foreground relative overflow-hidden">
      {/* 导航栏 */}
      <nav className="relative z-10 flex items-center justify-between px-6 md:px-12 py-4 max-w-6xl mx-auto">
        <div className="flex items-center gap-2.5">
          <img src={site.site_logo || defaultLogoUrl} alt="" className="w-8 h-8 rounded-sm object-cover" />
          <span className="text-base font-bold">{site.site_name || 'AirGate'}</span>
        </div>
        <div className="flex items-center gap-2">
          {showStatusEntry && (
            <HeroLink
              href="/status"
              className="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-muted hover:text-foreground transition-colors"
            >
              <Activity className="w-3.5 h-3.5" />
              {t('nav.status')}
            </HeroLink>
          )}
          <HeroLink
            href={docs.href}
            {...(docs.isExternal ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
            className="px-3 py-1.5 text-xs font-medium text-muted hover:text-foreground transition-colors"
          >
            {t('home.docs')}
          </HeroLink>
          <Button
            aria-label={theme === 'dark' ? '切换亮色模式' : '切换暗色模式'}
            isIconOnly
            size="sm"
            variant="ghost"
            onPress={toggleTheme}
          >
            {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
          </Button>
          <Button
            className="ml-2"
            size="sm"
            variant="primary"
            onPress={() => navigate({ to: isLoggedIn ? '/' : '/login' })}
          >
            {isLoggedIn ? t('home.go_dashboard') : t('home.login')}
          </Button>
        </div>
      </nav>

      {/* Hero：左文案 + 右 Endpoint 面板（pigcoder：首屏直接露出协议入口，不做居中营销 hero）*/}
      <section className="relative z-10 px-6 pt-12 pb-16 md:pt-20 md:pb-24 max-w-6xl mx-auto">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-10 lg:gap-12 items-center">
          {/* 左：定位 + CTA */}
          <div className="text-left">
            <div className="inline-flex items-center gap-2 px-4 py-1.5 rounded-[var(--radius)] text-xs font-medium mb-6 border border-border bg-surface">
              <Code className="w-3.5 h-3.5 text-[var(--accent)]" />
              <span className="text-muted">{t('home.badge')}</span>
            </div>
            <h1 className="text-4xl md:text-5xl font-bold leading-tight mb-4">
              {site.site_name || 'AirGate'}
            </h1>
            <p className="text-base md:text-lg text-muted max-w-lg mb-8 leading-relaxed">
              {site.site_subtitle || t('home.subtitle')}
            </p>
            <div className="flex flex-wrap items-center gap-3">
              <Button
                size="lg"
                variant="primary"
                onPress={() => navigate({ to: isLoggedIn ? '/' : '/login' })}
              >
                {isLoggedIn ? t('home.go_dashboard') : t('home.get_started')}
                <ArrowRight className="w-4 h-4" />
              </Button>
              <HeroLink
                href={docs.href}
                {...(docs.isExternal ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
                className="inline-flex items-center gap-2 px-6 py-2.5 text-sm font-medium rounded-[var(--radius)] border border-border text-muted hover:text-foreground hover:bg-default-hover transition-colors"
              >
                {t('home.view_docs')}
              </HeroLink>
            </div>
          </div>

          {/* 右：Endpoint 面板 */}
          <Card className="w-full">
            <Card.Content className="p-5 space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Globe className="w-4 h-4 text-accent" />
                  <h2 className="text-sm font-semibold text-foreground">{t('home.endpoints_title')}</h2>
                </div>
                <span className="text-[11px] text-muted">{t('home.endpoints_hint')}</span>
              </div>

              <div className="space-y-2">
                {endpoints.map((ep) => (
                  <button
                    key={ep.protocol}
                    type="button"
                    onClick={() => copy(ep.protocol, ep.url)}
                    aria-label={t('common.copy')}
                    className="group flex w-full items-center gap-3 rounded-[var(--radius)] border border-border bg-background px-3 py-2.5 text-left transition-colors hover:border-accent/40"
                  >
                    <span className="w-[4.5rem] shrink-0 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted">
                      {ep.protocol}
                    </span>
                    <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-foreground">{ep.url}</code>
                    {copiedKey === ep.protocol ? (
                      <Check className="w-3.5 h-3.5 shrink-0 text-success" />
                    ) : (
                      <Copy className="w-3.5 h-3.5 shrink-0 text-muted transition-colors group-hover:text-accent" />
                    )}
                  </button>
                ))}
              </div>

              {/* 快速接入 */}
              <div className="pt-1">
                <div className="mb-1.5 flex items-center justify-between">
                  <span className="text-[11px] font-medium text-muted">{t('home.quick_start')}</span>
                  <button
                    type="button"
                    onClick={() => copy('quickstart', quickStart)}
                    aria-label={t('common.copy')}
                    className="inline-flex items-center gap-1 text-[11px] text-muted transition-colors hover:text-accent"
                  >
                    {copiedKey === 'quickstart' ? (
                      <><Check className="w-3 h-3 text-success" />{t('common.copy')}</>
                    ) : (
                      <><Copy className="w-3 h-3" />{t('common.copy')}</>
                    )}
                  </button>
                </div>
                <pre className="overflow-x-auto rounded-[var(--radius)] border border-border bg-background p-3 font-mono text-[12px] leading-relaxed text-foreground">
{quickStart}
                </pre>
              </div>
            </Card.Content>
          </Card>
        </div>
      </section>

      {/* 能力区 */}
      <section className="relative z-10 px-6 pb-20 max-w-6xl mx-auto">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {features.map((f) => (
            <Card key={f.titleKey}>
              <Card.Content className="p-5">
                <div className="flex items-center justify-center w-10 h-10 rounded-[var(--radius)] bg-accent-soft text-accent mb-3">
                  {f.icon}
                </div>
                <h3 className="text-sm font-semibold mb-1">{t(f.titleKey)}</h3>
                <p className="text-xs text-muted leading-relaxed">{t(f.descKey)}</p>
              </Card.Content>
            </Card>
          ))}
        </div>
      </section>

      {/* 自定义 HTML 内容 */}
      {site.home_content && (
        <section className="relative z-10 px-6 pb-16 max-w-4xl mx-auto">
          <div
            className="prose prose-sm dark:prose-invert max-w-none text-muted"
            dangerouslySetInnerHTML={{ __html: site.home_content }}
          />
        </section>
      )}

      {/* 联系方式 & 底部 */}
      <footer className="relative z-10 border-t border-[var(--border)] py-8 text-center">
        <div className="flex items-center justify-center gap-4 text-xs text-muted">
          <span>© {new Date().getFullYear()} {site.site_name || 'AirGate'} · {t('home.copyright')}</span>
          {site.contact_info && (
            <>
              <span className="w-px h-3 bg-[var(--border)]" />
              <span>{site.contact_info}</span>
            </>
          )}
        </div>
      </footer>
    </div>
  );
}
