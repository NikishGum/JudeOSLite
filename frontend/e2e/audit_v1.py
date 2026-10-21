"""Compatibility entry point: current audit UX is covered by UX-HISTORY #220."""
import runpy
from pathlib import Path

runpy.run_path(str(Path(__file__).with_name('history_search.py')), run_name='__main__')
