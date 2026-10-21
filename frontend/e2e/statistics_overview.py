"""#221 UI contracts and pinned visual pairs. Synthetic API fixtures; Go/PG is tested separately."""

import json
import os
from datetime import datetime
from pathlib import Path
from urllib.parse import urlparse, parse_qs
from zoneinfo import ZoneInfo
from playwright.sync_api import sync_playwright, expect
from screenshot_redesign import Fixture, ROOT, uid
from runtime_ui import wait_for_api_idle, observe_api

ENGINE = os.environ.get("PWA_BROWSER", "chromium")
OUT = Path(os.environ.get("STATS_EVIDENCE", "/tmp/stats221-" + ENGINE))
OUT.mkdir(parents=True, exist_ok=True)
REF = os.environ.get("STATS_REFERENCE")
FIELDS = (
    "closed",
    "planned",
    "open",
    "present",
    "absent",
    "unknown",
    "illness",
    "excused",
)
VALUES = [
    ("Брест · Восток", 77, 9, 1, 1232, 308, 80, 154, 74),
    ("Брест · Центр", 60, 8, 0, 985, 165, 25, 100, 40),
    ("Минск · Южный", 44, 7, 3, 620, 230, 70, 100, 60),
    ("Пинск · Центр", 34, 6, 0, 546, 104, 20, 64, 20),
    ("Кобрин · Центр", 22, 6, 2, 330, 126, 50, 44, 32),
    ("Брест · Заречье", 0, 4, 0, 0, 0, 0, 0, 0),
]


def totals(values):
    result = dict(zip(FIELDS, values))
    result["percentage"] = (
        result["present"] * 100 / (result["present"] + result["absent"])
        if result["present"] + result["absent"]
        else None
    )
    return result


class StatisticsFixture(Fixture):
    def __init__(self):
        super().__init__()
        self.error = 0
        self.requests = []
        self.hold = False
        self.pending = []
        self.rows = [
            dict(
                totals(row[1:]),
                clubId=uid(100 + i),
                clubName=row[0],
                clubArchived=False,
                groupId=None,
                groupName=None,
                groupArchived=False,
            )
            for i, row in enumerate(VALUES)
        ]
        self.groups = [
            dict(row, groupId=uid(200 + i), groupName="7–10 лет")
            for i, row in enumerate(self.rows)
        ]

    def route(self, route):
        path = urlparse(route.request.url).path
        q = parse_qs(urlparse(route.request.url).query)
        if path.startswith("/api/reports/"):
            self.requests.append(route.request.url)
            if self.hold:
                self.pending.append(route)
                return
            if self.error:
                route.fulfill(
                    status=self.error,
                    json={
                        "error": {
                            "code": "unavailable",
                            "message": (
                                "Нет доступа."
                                if self.error in (401, 403, 404, 410)
                                else "Временно недоступно."
                            ),
                        }
                    },
                )
                return
            if path.endswith("filters"):
                route.fulfill(
                    json={
                        "clubs": [
                            dict(
                                id=r["clubId"],
                                name=r["clubName"],
                                archived=r["clubArchived"],
                            )
                            for r in self.rows
                        ],
                        "groups": [
                            dict(
                                id=r["groupId"],
                                clubId=r["clubId"],
                                name=r["groupName"],
                                archived=r["groupArchived"],
                            )
                            for r in self.groups
                        ],
                    }
                )
                return
            groups = [
                r
                for r in self.groups
                if (not q.get("clubId") or r["clubId"] == q["clubId"][0])
                and (not q.get("groupId") or r["groupId"] == q["groupId"][0])
            ]
            rows = [
                r for r in self.rows if any(g["clubId"] == r["clubId"] for g in groups)
            ]
            total = totals([sum(r[f] for r in groups) for f in FIELDS])
            opened = [
                dict(
                    id=uid(300 + i * 10 + j),
                    clubId=r["clubId"],
                    groupId=r["groupId"],
                    date="2026-10-20",
                    startsAt="2026-10-20T10:00:00Z",
                    timezone="Europe/Minsk",
                )
                for i, r in enumerate(groups)
                for j in range(r["open"])
            ]
            route.fulfill(
                json={
                    "from": q["from"][0],
                    "until": q["until"][0],
                    "total": total,
                    "clubs": rows,
                    "groups": groups,
                    "openSessions": opened,
                }
            )
            return
        super().route(route)

    def release(self):
        self.hold = False
        pending, self.pending = self.pending, []
        for route in pending:
            self.route(route)


with sync_playwright() as p:
    browser = getattr(p, ENGINE).launch()
    errors = []
    f = StatisticsFixture()
    context = browser.new_context(
        viewport={"width": 1528, "height": 1000},
        service_workers="block",
        reduced_motion="reduce",
        timezone_id="Pacific/Honolulu",
    )
    context.add_init_script("delete Navigator.prototype.serviceWorker")
    context.route("**/api/**", f.route)
    page = context.new_page()
    page.on("pageerror", lambda e: errors.append(str(e)))
    observe_api(page)
    page.clock.set_fixed_time(
        datetime(2026, 10, 20, 12, tzinfo=ZoneInfo("Europe/Minsk"))
    )
    page.goto(ROOT + "/statistics")
    screen = page.locator(".statistics-overview")
    expect(screen.locator('[data-metric="closed"]')).to_have_text("237")
    wait_for_api_idle(page)
    # Applied range stays intact until an explicit valid submit, and quick periods retain the manual draft.
    screen.get_by_role("button", name="Период", exact=True).click()
    screen.get_by_label("Период с", exact=True).fill("2026-10-21")
    screen.get_by_label("Период по", exact=True).fill("2026-10-20")
    before = len(f.requests)
    screen.get_by_role("button", name="Применить", exact=True).click()
    expect(screen.get_by_role("alert")).to_contain_text("начало периода")
    assert len(f.requests) == before
    expect(screen.locator('[data-metric="closed"]')).to_have_text("237")
    screen.get_by_label("Период с", exact=True).fill("2026-09-01")
    screen.get_by_role("button", name="Применить", exact=True).click()
    assert "reportFrom=2026-09-01" in page.url
    screen.get_by_role("button", name="Неделя", exact=True).click()
    assert "reportFrom=2026-10-19" in page.url
    screen.get_by_role("button", name="Период", exact=True).click()
    expect(screen.get_by_label("Период с", exact=True)).to_have_value("2026-09-01")
    screen.get_by_label("Период с", exact=True).fill("2026-10-01")
    screen.get_by_role("button", name="Применить", exact=True).click()
    wait_for_api_idle(page)
    # Exact open list belongs to the applied context; low rows exclude the zero denominator.
    screen.get_by_role("button", name="Показать эти филиалы", exact=True).click()
    expect(screen.locator(".open-journal")).to_have_count(6)
    expect(screen.locator(".rank-row")).to_have_count(3)
    link = screen.get_by_role("link", name="Открыть журнал", exact=False).first
    href = link.get_attribute("href")
    assert (
        "status=open" in href
        and "reportFrom=2026-10-01" in href
        and "reportDetail=open" in href
    )
    screen.get_by_role("button", name="К обзору сети", exact=True).click()
    screen.get_by_role("button", name="Сравнить эти филиалы", exact=True).click()
    expect(screen.locator(".rank-row")).to_have_count(2)
    screen.get_by_role("button", name="К обзору сети", exact=True).click()
    # Same-name groups use UUIDs; branch selection clears the group.
    first = f.rows[0]
    screen.get_by_role(
        "button", name="Группы: " + first["clubName"], exact=True
    ).click()
    screen.locator('[data-group-id="' + f.groups[0]["groupId"] + '"] button').click()
    expect(screen.get_by_label("Филиал отчёта", exact=True)).to_have_value(
        first["clubId"]
    )
    expect(screen.get_by_label("Группа отчёта", exact=True)).to_have_value(
        f.groups[0]["groupId"]
    )
    screen.get_by_label("Филиал отчёта", exact=True).select_option("")
    expect(screen.get_by_label("Группа отчёта", exact=True)).to_have_value("")
    expect(screen.locator('[data-metric="closed"]')).to_have_text("237")
    wait_for_api_idle(page)
    # Refresh retains DOM, open details, draft, focus and sorting. A transient error preserves totals.
    screen.get_by_role(
        "button", name="Группы: " + first["clubName"], exact=True
    ).click()
    screen.get_by_label("Порядок филиалов", exact=True).select_option("open")
    screen.get_by_role("button", name="Период", exact=True).click()
    screen.get_by_label("Период с", exact=True).fill("2026-08-01")
    screen.get_by_label("Период с", exact=True).focus()
    page.evaluate(
        'window.statsNodes=[document.querySelector(".summary-panel"),document.querySelector(".rank-row")];window.statsFocus=document.activeElement'
    )
    f.hold = True
    page.evaluate('dispatchEvent(new Event("focus"))')
    page.wait_for_function(
        'document.querySelector(".statistics-overview").getAttribute("aria-busy")==="true"'
    )
    assert page.evaluate(
        "statsNodes.every(n=>n.isConnected) && statsFocus===document.activeElement"
    )
    f.release()
    expect(screen).to_have_attribute("aria-busy", "false")
    expect(
        screen.get_by_role("button", name="Группы: " + first["clubName"], exact=True)
    ).to_have_attribute("aria-expanded", "true")
    expect(screen.get_by_label("Период с", exact=True)).to_have_value("2026-08-01")
    assert page.evaluate(
        "statsNodes.every(n=>n.isConnected) && statsFocus===document.activeElement"
    )
    f.error = 503
    screen.get_by_role("button", name="Обновить статистику", exact=True).click()
    expect(screen.get_by_role("alert")).to_contain_text("Временно")
    expect(screen.locator('[data-metric="closed"]')).to_have_text("237")
    assert page.evaluate("statsNodes.every(n=>n.isConnected)")
    f.error = 0
    screen.get_by_role(
        "button", name="Повторить загрузку статистики", exact=True
    ).click()
    expect(screen.get_by_role("alert")).to_have_count(0)
    screen.get_by_role("button", name="Период", exact=True).click()
    screen.get_by_role(
        "button", name="Группы: " + first["clubName"], exact=True
    ).click()
    screen.get_by_label("Порядок филиалов", exact=True).select_option("rate")
    # Capture all required content widths in one batch. Paired report widths are measured, not inferred from sidebar/window size.
    reference = (
        browser.new_page(
            viewport={"width": 1280, "height": 1000}, reduced_motion="reduce"
        )
        if REF
        else None
    )
    if reference:
        reference.goto(REF)
        reference.evaluate("document.fonts.ready")
    page.evaluate("document.fonts.ready")
    geometry = []
    for content in (319, 320, 390, 768, 1280):
        page.set_viewport_size(
            {"width": content + 312 if content > 760 else content + 32, "height": 1000}
        )
        page.locator(".statistics-frame").evaluate(
            '(el,w)=>el.style.width=w+"px"', content
        )
        assert screen.bounding_box()["width"] == content
        assert page.evaluate(
            "document.documentElement.scrollWidth<=innerWidth"
        ), content
        for control in screen.locator(
            "button:visible,input:visible,select:visible,summary:visible,a:visible"
        ).all():
            rect = control.bounding_box()
            assert rect["height"] >= 44 and rect["width"] >= 44, (
                content,
                control.text_content(),
                rect,
            )
        page.evaluate("scrollTo(0,0)")
        page.screenshot(path=str(OUT / f"shell-content-{content}.png"), full_page=True)
        geometry.append(dict(content=content, actual=screen.bounding_box()["width"]))
    if reference:
        for label, window, content in (("desktop", 1280, 1216), ("mobile", 390, 358)):
            reference.set_viewport_size({"width": window, "height": 1000})
            page.set_viewport_size(
                {"width": content + 312 if window > 760 else window, "height": 1000}
            )
            page.locator(".statistics-frame").evaluate(
                '(el,w)=>el.style.width=w+"px"', content
            )
            page.mouse.move(0, 0)
            reference.mouse.move(0, 0)
            reference.locator("#report").screenshot(
                path=str(OUT / f"reference-{label}.png")
            )
            screen.locator(".overview-content").screenshot(
                path=str(OUT / f"implementation-{label}.png"),
                style=".bottom-navigation { visibility:hidden !important; }",
            )
            assert (
                abs(
                    reference.locator("#report").bounding_box()["width"]
                    - screen.locator(".overview-content").bounding_box()["width"]
                )
                < 1
            )
    page.locator(".statistics-frame").evaluate('el=>el.style.removeProperty("width")')
    page.set_viewport_size({"width": 1440, "height": 1000})
    page.evaluate("scrollTo(0,0)")
    page.screenshot(path=str(OUT / "shell-1440.png"), full_page=True)
    # Finance survives the integration.
    page.get_by_role("tab", name="Финансы", exact=True).click()
    expect(
        page.get_by_role("heading", name="Финансовый отчёт", exact=True)
    ).to_be_visible()
    page.get_by_role("tab", name="Посещаемость", exact=True).click()
    # Same-name active groups within one branch are distinct in both actionable lists.
    duplicate = dict(f.groups[0], **totals([0] * 8))
    duplicate["groupId"] = uid(999)
    f.groups.append(duplicate)
    page.evaluate('dispatchEvent(new Event("focus"))')
    expect(
        screen.get_by_label("Группа отчёта", exact=True).locator("option")
    ).to_have_count(8)
    screen.get_by_role(
        "button", name="Группы: " + first["clubName"], exact=True
    ).click()
    for group in (f.groups[0], duplicate):
        expect(
            screen.locator('[data-group-id="' + group["groupId"] + '"] button')
        ).to_contain_text(group["groupId"][-8:])
    screen.get_by_role("button", name="Показать эти филиалы", exact=True).click()
    expect(screen.locator(".open-journal h3").first).to_contain_text(
        f.groups[0]["groupId"][-8:]
    )
    screen.get_by_role("button", name="К обзору сети", exact=True).click()
    # Archive removes working actions without dropping historical rows or counts.
    f.rows[0]["clubArchived"] = True
    for group in f.groups:
        if group["clubId"] == first["clubId"]:
            group["clubArchived"] = True
    page.evaluate('dispatchEvent(new Event("focus"))')
    expect(
        screen.get_by_role(
            "button", name="Группы: " + first["clubName"] + " · в архиве", exact=True
        )
    ).to_be_visible()
    screen.get_by_role("button", name="Показать эти филиалы", exact=True).click()
    expect(screen.locator(".open-journal")).to_have_count(6)
    expect(
        screen.get_by_role("link", name="Открыть журнал", exact=False)
    ).to_have_count(5)
    screen.get_by_role("button", name="К обзору сети", exact=True).click()
    # Long labels stay within the content box, including the smallest required width.
    original_name = first["clubName"]
    f.rows[0]["clubName"] = "Филиал с очень длинным названием " * 6
    page.evaluate('dispatchEvent(new Event("focus"))')
    expect(
        screen.locator(".branch-name").filter(
            has_text="Филиал с очень длинным названием"
        )
    ).to_be_visible()
    for content in (319, 390, 768, 1280):
        page.set_viewport_size(
            {"width": content + 312 if content > 760 else content + 32, "height": 1000}
        )
        page.locator(".statistics-frame").evaluate(
            '(el,w)=>el.style.width=w+"px"', content
        )
        assert page.evaluate("document.documentElement.scrollWidth<=innerWidth"), (
            "long labels",
            content,
        )
    f.rows[0]["clubName"] = original_name
    # Denials clear report and filter names; no transient-error fallback at security boundaries.
    for status in (403, 404, 410):
        f.error = status
        screen.get_by_role("button", name="Обновить статистику", exact=True).click()
        expect(screen.locator('[data-metric="closed"]')).to_have_count(0)
        assert first["clubName"] not in screen.inner_text()
        f.error = 0
        screen.get_by_role(
            "button", name="Повторить загрузку статистики", exact=True
        ).click()
        expect(screen.locator('[data-metric="closed"]')).to_have_text("237")
    f.rows = []
    f.groups = []
    page.evaluate('dispatchEvent(new Event("focus"))')
    expect(
        screen.get_by_role("heading", name="В этом периоде занятий нет", exact=True)
    ).to_be_visible()
    expect(screen.locator('[data-metric="percentage"]')).to_have_text("—")
    expect(screen.locator(".findings-panel")).to_have_count(0)
    assert not errors, errors
    (OUT / "result.json").write_text(
        json.dumps(
            dict(
                engine=ENGINE,
                version=browser.version,
                result="PASS",
                geometry=geometry,
                api="synthetic fixtures; real API verified by redesign_reports and reports Go integration",
            ),
            ensure_ascii=False,
            indent=2,
        )
        + "\n"
    )
    print((OUT / "result.json").read_text())
    browser.close()
