"""The approved access card, shared by existing regression and targeted checks."""
from playwright.sync_api import expect


def select_employee(ui, employee_id):
    back = ui.get_by_role('button', name='К списку сотрудников', exact=True)
    if back.is_visible():
        back.click()
    ui.locator(f'[data-person-id="{employee_id}"]').click()
    expect(ui.locator('.staff-access-card')).to_be_visible()


def add_assignment(ui, employee_id, club_id, role='trainer'):
    select_employee(ui, employee_id)
    ui.locator('.staff-access-card').get_by_role('button', name='Добавить назначение', exact=True).click()
    panel = ui.get_by_role('dialog')
    panel.get_by_label('Роль', exact=True).select_option(role)
    panel.get_by_label('Филиал назначения', exact=True).select_option(club_id)
    panel.get_by_role('button', name='Добавить назначение', exact=True).click()
    expect(panel).to_have_count(0)


def discard_staff_form(ui):
    ui.get_by_role('dialog').get_by_role('button', name='Отмена', exact=True).click()
    confirm = ui.get_by_role('dialog', name='Есть несохранённые изменения', exact=True)
    if confirm.count():
        confirm.get_by_role('button', name='Перейти без сохранения', exact=True).click()
    expect(ui.get_by_role('dialog')).to_have_count(0)


def toggle_access(ui, employee_id, enabled):
    select_employee(ui, employee_id)
    ui.locator('.staff-access-card').get_by_role('button', name='Включить доступ' if enabled else 'Отключить весь доступ', exact=True).click()
    ui.get_by_role('dialog').get_by_role('button', name='Подтвердить включение' if enabled else 'Подтвердить отключение', exact=True).click()
    expect(ui.get_by_role('dialog')).to_have_count(0)
