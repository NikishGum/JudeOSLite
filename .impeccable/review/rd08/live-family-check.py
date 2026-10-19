from pathlib import Path
source_path=Path(__file__).resolve().parents[3]/'backend/tests/browser/families.py'
source=source_path.read_text()
marker="        for member in children[:2]:"
extra='''        # Frontend integration on the backend developer's real isolated fixtures.
        page.wait_for_load_state('networkidle')
        page.goto(ROOT+f'/clubs/{club}/families')
        expect(page.get_by_role('button',name='Synthetic Family 0',exact=True)).to_be_visible()
        page.get_by_role('button',name='Synthetic Family 0',exact=True).click()
        expect(page.get_by_role('dialog')).to_contain_text('Synthetic Parent')
        page.get_by_role('tab',name='Доступ родителей',exact=True).click()
        expect(page.get_by_role('dialog')).to_contain_text('parent0')
        page.get_by_role('button',name='Закрыть семью',exact=True).click()
        parent.goto(ROOT+f'/clubs/{club}/families')
        expect(parent.locator('.portal-family')).to_contain_text('Synthetic Family 0')
        expect(parent.locator('.child-card')).to_have_count(2)
        expect(parent.get_by_label('Филиал',exact=True)).to_have_count(0)
        parent.locator('.child-card').first.click()
        expect(parent.locator('.member-profile')).to_contain_text('Synthetic Child')
        print('RD08 LIVE UI: real owner family/access card and parent two-child profile PASS',flush=True)
'''
assert source.count(marker)==1
source=source.replace(marker,extra+marker)
marker="        api(parent,'GET',f'/api/clubs/{club}/member-profiles/{children[0]}',expected=404)"
extra='''        parent.evaluate('window.dispatchEvent(new Event("focus"))')
        expect(parent.locator('.member-profile')).to_have_count(0)
        expect(parent.locator('.child-card')).to_have_count(1)
        print('RD08 LIVE UI: real same-family child unlink clears open private profile PASS',flush=True)
'''
assert source.count(marker)==1
source=source.replace(marker,extra+marker)
exec(compile(source,str(source_path),'exec'),{'__file__':str(source_path),'__name__':'__main__'})
