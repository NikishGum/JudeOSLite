"""Actions moved into the real profile menu by the approved shadcn shell."""
def signout(page):
    page.get_by_role('button', name='Меню профиля', exact=True).click()
    page.get_by_role('menuitem', name='Выйти', exact=True).click()


def assert_control_target(control, width):
    """The approved desktop toggle/date group is compact; mobile stays44px."""
    classes = (control.get_attribute('class') or '').split()
    minimum = 44
    if width >= 768 and control.get_attribute('data-slot') == 'sidebar-trigger':
        minimum = 36
    elif width >= 768 and ('date-step' in classes or 'date-select-trigger' in classes):
        minimum = 32
    rect = control.bounding_box()
    assert rect and rect['height'] >= minimum and rect['width'] >= minimum, (width, minimum, control.text_content(), control.get_attribute('class'), control.get_attribute('aria-label'), rect)


def assert_page_targets(page, width, extra_selector=''):
    settle_shell(page, width)
    # Read one DOM snapshot: responsive React mounts can invalidate nth locators.
    controls = page.locator('button:visible,input:visible,select:visible,summary:visible,.workspace-navigation a:visible'+extra_selector).evaluate_all('''nodes => nodes.filter(n=>n.isConnected && n.getClientRects().length && n.getBoundingClientRect().width>0 && n.getBoundingClientRect().height>0 && !['checkbox','radio'].includes(n.type)).map(n=>({
      label:n.getAttribute('aria-label')||n.textContent, classes:[...n.classList], slot:n.dataset.slot,
      width:n.getBoundingClientRect().width,height:n.getBoundingClientRect().height
    }))''')
    for control in controls:
        minimum = 44
        if width >= 768 and control['slot'] == 'sidebar-trigger':
            minimum = 36
        elif width >= 768 and any(c in control['classes'] for c in ('date-step', 'date-select-trigger')):
            minimum = 32
        assert control['width'] >= minimum and control['height'] >= minimum, (width, minimum, control)


def settle_shell(page, width):
    # CSS changes synchronously; useIsMobile mounts/removes Sidebar on the next render.
    if page.locator('.app-shell').count():
        page.wait_for_function('''desktop => {
          const sidebar=document.querySelector('[data-slot="sidebar"]');
          const profiles=[...document.querySelectorAll('[aria-label="Меню профиля"]')];
          return Boolean(sidebar)===desktop && profiles.length===1 && profiles[0].getBoundingClientRect().width>0;
        }''', arg=width >= 768)
