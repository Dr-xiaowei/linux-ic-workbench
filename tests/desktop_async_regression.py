# -*- coding: utf-8 -*-
"""Deterministic delayed-response and backpressure regression; Python 2.6+."""
from __future__ import unicode_literals
import os, sys, unittest
sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), 'desktop'))
import runtime
from client import Workbench

USE_TK = '--tk' in sys.argv
if USE_TK: sys.argv.remove('--tk')

class Root(object):
    def after(self, *args): pass

class Field(object):
    def __init__(self, value=''): self.value = value
    def get(self, *args): return self.value
    def cget(self, key): return 'normal'
    def configure(self, **kw): pass
    def delete(self, *args): self.value = ''
    def insert(self, index, value): self.value = value

class Rows(object):
    def __init__(self): self.rows = []; self.selection = None
    def set(self, rows, values): self.rows = rows; self.selection = None
    def selected(self): return self.rows[self.selection] if self.selection is not None else None
    def select(self, index): self.selection = index

class DesktopAsyncTests(unittest.TestCase):
    def setUp(self):
        a = Workbench.__new__(Workbench); self.app = a
        a.root = runtime.tk.Tk() if USE_TK else Root()
        a.page = 'projects'; a.generation = 1; a.lang = 'zh'
        a.project = {'id': 'p', 'root': '/project'}
        a.file_root = '/project'; a.file_path = ''; a.file_offset = 0
        a.errors = []; a.info = lambda s: None; a.error = a.errors.append
        a.capture = lambda: None; a.pending = []
        a.action = lambda name, data=None, callback=None, failure=None: a.pending.append((name, data, callback))
        if USE_TK:
            a.file_table = runtime.Table(a.root, ['File', 'Note', 'Description'])
            a.file_table.pack(); a.widgets = {'preview': runtime.tk.Text(a.root), 'query': runtime.tk.Entry(a.root)}
            for w in a.widgets.values(): w.pack()
            a.buttons = dict((k, runtime.tk.Button(a.root)) for k in ('next_files', 'search_more'))
            a.root.update()
        else:
            a.file_table = Rows(); a.widgets = {'preview': Field(), 'query': Field()}
            a.buttons = dict((k, Field()) for k in ('next_files', 'search_more'))
        self.query('gain')

    def tearDown(self):
        if USE_TK: self.app.root.destroy()

    def query(self, value):
        w = self.app.widgets['query']; w.delete(0, 'end'); w.insert(0, value)

    def reply(self, index, value): self.app.pending[index][2](value)
    def entries(self, name): return {'entries': [{'name': name, 'path': name}]}
    def hits(self, name, more=False): return {'hits': [{'path': name}], 'more': more, 'cursor': 'cursor-old'}
    def preview(self): return runtime.text_value(self.app.widgets['preview'])

    def test_save_drains_changes_arriving_during_previous_write(self):
        a = self.app; a.saving = False; a.dirty = True; a.epoch = 1; a.save_error = False
        a.workspace = {'revision': 1}; a.data = {'calculator': {}}
        pending = []
        a.request = lambda route, payload, done, failed: pending.append((payload, done))
        a.save_workspace()
        a.data['calculator']['result'] = {'value': 0.1}; a.epoch = 2
        pending[0][1]({'revision': 2})
        self.assertTrue(a.saving)
        self.assertEqual(len(pending), 2)
        self.assertEqual(pending[1][0]['data']['calculator']['result']['value'], 0.1)
        pending[1][1]({'revision': 3})
        self.assertFalse(a.saving); self.assertFalse(a.dirty)

    def test_queue_full_restores_save_and_keeps_unsaved_data(self):
        a = self.app; a.saving = False; a.dirty = True; a.save_error = False; a.epoch = 2
        a.workspace = {'revision': 4}; a.data = {'codeDrafts': {'python': {'code': 'my work'}}}
        w = runtime.Workers.__new__(runtime.Workers)
        w.root = Root(); w.pending = runtime.queue.Queue(1); w.results = runtime.queue.Queue(); w.closed = False
        w.on_error = a.error; a.workers = w
        w.pending.put((None, None, None))
        a.save_workspace()
        self.assertFalse(a.saving)
        self.assertTrue(a.dirty)
        self.assertTrue(a.save_error)
        self.assertEqual(1, len(a.errors))
        self.assertEqual('my work', a.data['codeDrafts']['python']['code'])
        # The user can retry without restarting the application.
        w.pending.get_nowait(); a.save_workspace()
        fn, callback, failure = w.pending.get_nowait()
        callback({'revision': 5})
        self.assertFalse(a.saving); self.assertFalse(a.dirty); self.assertFalse(a.save_error)
        self.assertEqual(5, a.workspace['revision'])

    def test_queue_full_default_handler_runs_once(self):
        w = runtime.Workers.__new__(runtime.Workers); errors = []
        w.pending = runtime.queue.Queue(1); w.pending.put(None); w.closed = False; w.on_error = errors.append
        w.submit(lambda: self.fail('rejected work executed'))
        self.assertEqual(1, len(errors))

    def test_old_directory_cannot_replace_new_directory(self):
        a = self.app; a.browse(); a.file_path = 'new'; a.browse()
        self.reply(1, self.entries('new-file')); self.reply(0, self.entries('old-file'))
        self.assertEqual('new-file', a.file_table.rows[0]['name'])

    def test_old_root_cannot_replace_new_root(self):
        a = self.app; a.browse(); a.change_root('/other-root')
        self.reply(1, self.entries('other-root-file')); self.reply(0, self.entries('wrong-root-file'))
        self.assertEqual('other-root-file', a.file_table.rows[0]['name'])

    def test_old_search_cannot_replace_new_search(self):
        a = self.app; a.search_files(''); self.query('power'); a.search_files('')
        self.reply(1, self.hits('power')); self.reply(0, self.hits('gain', True))
        self.assertEqual('power', a.file_table.rows[0]['path'])
        self.assertEqual('', a.search_cursor)

    def test_directory_response_cannot_replace_search(self):
        a = self.app; a.browse(); a.search_files('')
        self.reply(1, self.hits('search-result')); self.reply(0, self.entries('directory-result'))
        self.assertEqual('search-result', a.file_table.rows[0]['path'])

    def test_search_response_cannot_replace_directory(self):
        a = self.app; a.search_files(''); a.browse()
        self.reply(1, self.entries('directory-result')); self.reply(0, self.hits('search-result'))
        self.assertEqual('directory-result', a.file_table.rows[0]['name'])

    def test_changed_query_does_not_reuse_old_cursor(self):
        a = self.app; a.search_files(''); self.reply(0, self.hits('gain', True))
        self.query('power'); a.continue_search()
        self.assertEqual('', a.pending[1][1]['cursor'])

    def test_old_file_cannot_replace_new_file_preview(self):
        a = self.app; a.file_table.set([{'path': 'A'}], lambda r: [r['path'], '', '']); a.file_table.select(0); a.open_file()
        a.file_table.set([{'path': 'B'}], lambda r: [r['path'], '', '']); a.file_table.select(0); a.open_file()
        self.reply(1, {'content': 'B content'}); self.reply(0, {'content': 'A content'})
        self.assertEqual('B content', self.preview())

    def test_leaving_directory_invalidates_file_preview(self):
        a = self.app; a.file_table.set([{'path': 'A'}], lambda r: [r['path'], '', '']); a.file_table.select(0); a.open_file()
        a.file_path = 'elsewhere'; a.browse(); self.reply(1, self.entries('elsewhere'))
        self.reply(0, {'content': 'stale content'})
        self.assertEqual('', self.preview())

    def test_leaving_page_ignores_pending_results(self):
        a = self.app; a.browse(); a.page = 'code'; a.generation += 1
        self.reply(0, self.entries('old-file'))
        self.assertEqual([], a.file_table.rows)

    def test_refresh_preserves_file_identity_after_annotation(self):
        a = self.app; a.browse()
        self.reply(0, {'entries': [{'name': 'A', 'path': 'A'}, {'name': 'B', 'path': 'B'}]})
        a.file_table.select(1); a.browse()
        self.reply(1, {'entries': [{'name': 'B', 'path': 'B', 'notes': 'reviewed'}, {'name': 'A', 'path': 'A'}]})
        self.assertEqual('B', a.file_table.selected()['path'])
        a.open_file(); self.assertEqual('B', a.pending[2][1]['path'])

if __name__ == '__main__': unittest.main()
