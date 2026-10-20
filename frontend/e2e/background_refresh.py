"""Background reads must preserve the current view; API responses are synthetic."""
import os
from collections import Counter
from urllib.parse import urlparse

from playwright.sync_api import expect, sync_playwright
from runtime_ui import observe_api, wait_for_api_idle
from redesign_accessibility import contrast_audit
from screenshot_redesign import CLUB, ROOT, TODAY, Fixture, uid

ENGINE = os.environ.get('PWA_BROWSER', 'chromium')


class RefreshFixture(Fixture):
    def __init__(self):
        super().__init__()
        self.hold = False
        self.pending = []
        self.requests = Counter()
        self.failures = {}
        self.counter = 142

    def route(self, route):
        path = urlparse(route.request.url).path
        self.requests[path] += 1
        if self.hold and route.request.method == 'GET':
            self.pending.append(route)
        else:
            self.answer(route)

    def answer(self, route):
        path = urlparse(route.request.url).path
        status = self.failures.get(path)
        if status:
            route.fulfill(status=status, json={'error': {
                'code': 'access_denied' if status == 403 else 'unavailable',
                'message': 'Нет доступа.' if status == 403 else 'Временно недоступно.',
            }})
        elif path == '/api/audit/filters':
            route.fulfill(json={'isOwner': True, 'clubs': [], 'actions': ['created']})
        elif path == '/api/audit/events':
            route.fulfill(json={'items': [dict(
                id=uid(999), createdAt=TODAY+'T09:00:00+03:00', actorName='Synthetic Owner',
                actorDisabled=False, clubName=None, clubArchived=False,
                entityType='club', entityId=CLUB, action='created',
            )], 'nextCursor': ''})
        elif path == '/api/dashboard':
            # Keep the real fixture shape while allowing a changed server value.
            original = route.fulfill
            def fulfill(**kwargs):
                kwargs['json']['counters']['participants'] = self.counter
                original(**kwargs)
            route.fulfill = fulfill
            try:
                super().route(route)
            finally:
                route.fulfill = original
        else:
            super().route(route)

    def release(self):
        self.hold = False
        pending, self.pending = self.pending, []
        for route in pending:
            self.answer(route)


def pin(page, selectors):
    # Finish font/layout and focus scrolling before measuring a background read.
    page.evaluate('document.fonts.ready')
    page.clock.run_for(500)
    page.evaluate('''selectors => {
      window.refreshNodes = selectors.map(s => document.querySelector(s));
      if (refreshNodes.some(n => !n)) throw new Error('Missing test content');
      scrollTo({top: 300, behavior: 'instant'});
      window.refreshScroll = scrollY;
      window.refreshFocus = document.activeElement;
    }''', selectors)


def assert_preserved(page, focus=False, scroll=False):
    assert page.evaluate('refreshNodes.every(n => n.isConnected)'), 'Background read unmounted existing content'
    if focus:
        assert page.evaluate('document.activeElement === refreshFocus'), 'Background read stole focus'
    if scroll:
        position = page.evaluate('({before: refreshScroll, after: scrollY, fonts: document.fonts.status, width: innerWidth})')
        assert abs(position['after'] - position['before']) <= 1, ('Background read changed scroll position', position)


def main():
    errors = []
    with sync_playwright() as playwright:
        browser = getattr(playwright, ENGINE).launch(
            executable_path=os.environ.get('CHROMIUM_PATH') if ENGINE == 'chromium' else None,
        )
        for width in (1440, 390):
            fixture = RefreshFixture()
            context = browser.new_context(viewport={'width': width, 'height': 900}, service_workers='block', reduced_motion='reduce')
            context.add_init_script('delete Navigator.prototype.serviceWorker')
            context.route('**/api/**', fixture.route)
            page = context.new_page()
            page.on('pageerror', lambda error: errors.append(str(error)))
            observe_api(page)
            page.clock.install()

            def go(path):
                wait_for_api_idle(page)
                page.goto(ROOT+path)
                wait_for_api_idle(page)
                page.evaluate('document.fonts.ready')

            def settle():
                fixture.release()
                wait_for_api_idle(page)

            go('/')
            expect(page.locator('[data-counter=participants]')).to_have_text('142')
            expect(page.locator('.report-totals')).to_be_visible()
            page.locator('.dashboard-report-explanation summary').click()
            page.get_by_label('Период главной', exact=True).focus()
            pin(page, ['.dashboard-counters', '.report-totals', '.dashboard-change-list', '.dashboard-report-explanation'])
            before = fixture.requests['/api/notifications']
            fixture.counter = 143
            fixture.hold = True
            page.clock.run_for(30_100)
            page.wait_for_timeout(100)
            assert fixture.pending, 'Timer did not revalidate'
            assert_preserved(page, focus=True, scroll=True)
            assert fixture.requests['/api/notifications'] == before+1, 'Duplicate notification polling'
            settle()
            expect(page.locator('[data-counter=participants]')).to_have_text('143')
            assert_preserved(page, focus=True, scroll=True)
            assert page.locator('.dashboard-report-explanation').evaluate('n => n.open')

            # A focus/visibility burst and explicit refresh keep mounted content.
            for trigger in (
                "dispatchEvent(new Event('focus')); document.dispatchEvent(new Event('visibilitychange'))",
                "dispatchEvent(new Event('attendance-changed'))",
            ):
                fixture.hold = True
                page.evaluate(trigger)
                page.wait_for_timeout(100)
                assert_preserved(page, focus=True, scroll=True)
                settle()
                assert_preserved(page, focus=True, scroll=True)

            # Temporary auth, report and dashboard failures must not reset the app.
            fixture.failures = {path: 503 for path in (
                '/api/auth/me', '/api/dashboard', '/api/reports/attendance', '/api/audit/events',
            )}
            page.clock.run_for(30_100)
            wait_for_api_idle(page)
            assert_preserved(page)
            expect(page.get_by_role('button', name='Меню профиля', exact=True)).to_be_visible()
            assert page.get_by_label('Логин', exact=True).count() == 0
            fixture.failures = {}
            page.clock.run_for(30_100)
            wait_for_api_idle(page)
            assert_preserved(page)

            assert not contrast_audit(page)['failures']
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
            page.screenshot(path=f'/tmp/background-refresh-{ENGINE}-{width}.png', full_page=True)

            # Explicit selection changes clear the previous scope immediately.
            fixture.hold = True
            page.get_by_label('Период главной', exact=True).select_option('week')
            expect(page.locator('.report-totals')).to_have_count(0)
            settle()
            expect(page.locator('.report-totals')).to_be_visible()

            # A draft and a save with CSRF survive a failed session revalidation.
            go(f'/clubs/{CLUB}/families')
            page.get_by_role('button', name='Добавить семью', exact=True).click()
            dialog = page.get_by_role('dialog', name='Новая семья', exact=True)
            expect(dialog).to_be_visible()
            wait_for_api_idle(page)
            dialog.get_by_label('Название семьи', exact=True).fill('Сохранённый черновик')
            dialog.get_by_label('Контактное лицо', exact=True).fill('Тестовый родитель')
            pin(page, ['dialog[open]', 'dialog[open] input'])
            fixture.failures['/api/auth/me'] = 503
            page.clock.run_for(30_100)
            wait_for_api_idle(page)
            assert_preserved(page, focus=True)
            expect(dialog.get_by_label('Название семьи', exact=True)).to_have_value('Сохранённый черновик')
            fixture.failures = {}
            dialog.get_by_role('button', name='Сохранить семью', exact=True).click()
            expect(dialog).to_have_count(0)
            wait_for_api_idle(page)
            assert any(path.endswith('/families') and body.get('name') == 'Сохранённый черновик'
                       for path, body in fixture.mutations), 'Draft was not saved with its CSRF token'

            go('/history')
            expect(page.locator('.audit-events li')).to_have_count(1)
            page.locator('.audit-record-id summary').click()
            page.get_by_label('С даты', exact=True).fill('2026-10-01')
            page.get_by_label('С даты', exact=True).focus()
            pin(page, ['.audit-events', '.audit-record-id', '.audit-filters'])
            fixture.hold = True
            page.clock.run_for(30_100)
            page.wait_for_timeout(100)
            assert_preserved(page, focus=True, scroll=True)
            settle()
            assert_preserved(page, focus=True, scroll=True)
            expect(page.get_by_label('С даты', exact=True)).to_have_value('2026-10-01')
            assert page.locator('.audit-record-id').evaluate('n => n.open')

            fixture.failures['/api/audit/events'] = 503
            page.clock.run_for(30_100)
            wait_for_api_idle(page)
            assert_preserved(page)
            fixture.failures = {}
            page.clock.run_for(30_100)
            wait_for_api_idle(page)

            # Permission revocation and session expiry still remove private data.
            fixture.failures['/api/audit/events'] = 403
            page.clock.run_for(30_100)
            wait_for_api_idle(page)
            expect(page.locator('.audit-events')).to_have_count(0)
            fixture.failures = {'/api/auth/me': 401}
            page.evaluate("dispatchEvent(new Event('focus'))")
            wait_for_api_idle(page)
            expect(page.get_by_label('Логин', exact=True)).to_be_visible()
            expect(page.locator('.audit-events')).to_have_count(0)
            context.close()
        browser.close()
    assert not errors, errors
    print('Background refresh PASS:', ENGINE, 'desktop/mobile; timer, focus, explicit refresh, failure, scope and access')


if __name__ == '__main__':
    main()
