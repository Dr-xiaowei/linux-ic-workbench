# -*- coding: utf-8 -*-
"""IC Workbench native Tk desktop. No browser, HTML renderer or WebView."""
from __future__ import unicode_literals
import os, sys, json, time, copy, math, signal, uuid
from runtime import (tk, filedialog, messagebox, BG, SIDE, PAPER, GREEN, INK, MUTED,
                     API, Workers, Table, Lifecycle, pretty, text_value, set_text,
                     export_text, read_text, workspace_defaults, as_text)

PAGES = [('chat', 'AI 对话', 'AI Agent'), ('projects', '项目与文件', 'Projects & files'),
         ('software', '软件管理', 'Software'), ('plugins', '插件', 'Plugins'),
         ('monitor', '资源监测', 'Resources'), ('analysis', '文件分析', 'File analysis'),
         ('code', 'Code 平台', 'Code'), ('calculator', '计算器', 'Calculator'),
         ('eda', 'EDA 控制', 'EDA control'), ('settings', '设置与验证', 'Settings')]

from engineering import Engineering, diff_text, readable_chat

class Workbench(Engineering):
    def __init__(self, root, api, metadata, lifecycle=None):
        self.root, self.api, self.meta, self.lifecycle = root, api, metadata, lifecycle
        self.state = metadata['state']; self.lang = self.state.get('language', 'zh')
        self.workspace = api.request('workspace')
        self.data = workspace_defaults(self.workspace.get('data'))
        self.preferences = api.action('preferences')
        self.page = ''; self.widgets = {}; self.buttons = {}; self.generation = 0
        self.dirty = False; self.saving = False; self.epoch = 0; self.save_error = False
        self.session = (self.data.get('chat') or {}).get('session', '')
        self.agent = ''; self.agent_snapshot = None; self.eda_run = ''; self.eda_id = 'builtin'
        self.code_run = ''; self.code_language = self.data.get('codeLanguage') or 'systemverilog'
        self.project = None; self.file_root = ''; self.file_path = ''; self.file_offset = 0
        self.analysis = None; self.analysis_id = self.data.get('analysisID', '')
        self.wave_zoom = 1.0; self.wave_pan = 0.0; self.errors = []
        self.closed = False; self.polling = False; self.alarm = ''
        root.title('IC Workbench · Desktop'); root.geometry('1360x920'); root.minsize(900, 660)
        root.configure(bg=BG); root.option_add('*Font', ('DejaVu Sans', 11))
        root.option_add('*selectBackground', '#cfe4d5'); root.option_add('*selectForeground', INK)
        root.tk.call('ttk::style', 'theme', 'use', 'clam')
        root.tk.call('ttk::style', 'configure', 'Treeview', '-background', PAPER, '-fieldbackground', PAPER,
                     '-foreground', INK, '-rowheight', 30, '-borderwidth', 0)
        root.tk.call('ttk::style', 'configure', 'Treeview.Heading', '-background', SIDE, '-foreground', INK)
        self.workers = Workers(root, self.error)
        root.protocol('WM_DELETE_WINDOW', self.close)
        self.shell(); self.goto('chat'); root.after(1000, self.tick)
        self.load_history()

    def tr(self, zh, en):
        return en if self.lang == 'en' else zh

    def error(self, error):
        text = as_text(error)
        self.errors.append(text); self.status.set(text)
        messagebox.showerror(self.tr('操作未完成', 'Operation failed'), text, parent=self.root)

    def info(self, text):
        self.status.set(text)

    def shell(self):
        for child in self.root.winfo_children():
            child.destroy()
        self.side = tk.Frame(self.root, bg=SIDE, width=212)
        self.side.pack(side='left', fill='y'); self.side.pack_propagate(False)
        tk.Label(self.side, text='⌁ IC Workbench', bg=SIDE, fg=INK, font=('DejaVu Sans', 15, 'bold')).pack(padx=18, pady=(30, 24), anchor='w')
        self.nav = {}
        for key, zh, en in PAGES:
            b = tk.Button(self.side, text=self.tr(zh, en), command=lambda k=key: self.goto(k),
                          anchor='w', relief='flat', bd=0, bg=SIDE, fg=INK, activebackground='#d8e8db', padx=20, pady=9)
            b.pack(fill='x', padx=12, pady=2); self.nav[key] = b
        tk.Label(self.side, text='LINUX DESKTOP\n0.8.0-rc3', bg=SIDE, fg=MUTED, justify='left').pack(side='bottom', anchor='w', padx=22, pady=22)
        self.main = tk.Frame(self.root, bg=BG); self.main.pack(side='left', fill='both', expand=True)
        top = tk.Frame(self.main, bg=BG); top.pack(fill='x', padx=28, pady=16)
        self.crumb = tk.Label(top, bg=BG, fg=MUTED); self.crumb.pack(side='left')
        self.button(top, 'language', 'EN / 中文', self.switch_language).pack(side='right', padx=5)
        self.button(top, 'save', self.tr('保存到本机', 'Save locally'), self.save_workspace).pack(side='right', padx=5)
        self.button(top, 'recovery', self.tr('恢复草稿', 'Recover drafts'), self.recovery).pack(side='right', padx=5)
        self.status = tk.StringVar(value=self.tr('就绪 · 原生桌面客户端', 'Ready · native desktop client'))
        tk.Label(self.main, textvariable=self.status, bg=SIDE, fg=GREEN, anchor='w', padx=20, pady=8, wraplength=1050).pack(side='bottom', fill='x')
        viewport = tk.Frame(self.main, bg=BG); viewport.pack(fill='both', expand=True, padx=20, pady=10)
        self.canvas = tk.Canvas(viewport, bg=BG, highlightthickness=0)
        vertical = tk.Scrollbar(viewport, command=self.canvas.yview); vertical.pack(side='right', fill='y')
        horizontal = tk.Scrollbar(viewport, orient='horizontal', command=self.canvas.xview); horizontal.pack(side='bottom', fill='x')
        self.canvas.pack(fill='both', expand=True); self.canvas.configure(yscrollcommand=vertical.set, xscrollcommand=horizontal.set)
        self.content = tk.Frame(self.canvas, bg=BG)
        self.content_item = self.canvas.create_window(0, 0, window=self.content, anchor='nw')
        def layout(event=None):
            if not self.canvas.winfo_exists(): return
            width = max(820, self.canvas.winfo_width())
            height = max(self.canvas.winfo_height(), self.content.winfo_reqheight())
            self.canvas.itemconfigure(self.content_item, width=width, height=height)
            self.canvas.configure(scrollregion=(0, 0, width, height))
        self.canvas.bind('<Configure>', layout); self.content.bind('<Configure>', layout)

    def button(self, parent, key, label, command, primary=False):
        b = tk.Button(parent, text=label, command=lambda: self.safe(command), bg=GREEN if primary else PAPER,
                      fg='white' if primary else INK, activebackground='#cfe4d5', relief='flat', bd=0, padx=13, pady=9)
        self.buttons[key] = b
        return b

    def safe(self, fn):
        try:
            return fn()
        except Exception as error:
            self.error(error)

    def row(self, parent=None):
        f = tk.Frame(parent or self.content, bg=BG); f.pack(fill='x', pady=6); return f

    def label(self, parent, text):
        w = tk.Label(parent, text=text, bg=BG, fg=MUTED, anchor='w'); w.pack(anchor='w', pady=(5, 4)); return w

    def entry(self, parent, key, value='', width=None):
        w = tk.Entry(parent, bg=PAPER, fg=INK, relief='flat', highlightthickness=1, highlightbackground='#dae3d9')
        if width:
            w.configure(width=width)
        w.insert(0, str(value) if not isinstance(value, type('')) else value)
        self.widgets[key] = w; return w

    def editor(self, parent, key, value='', height=7, readonly=False):
        frame = tk.Frame(parent, bg=PAPER)
        w = tk.Text(frame, height=height, width=1, wrap='word', bg=PAPER, fg=INK, relief='flat', padx=12, pady=10, undo=True)
        scroll = tk.Scrollbar(frame, command=w.yview); scroll.pack(side='right', fill='y')
        w.configure(yscrollcommand=scroll.set); w.pack(side='left', fill='both', expand=True)
        w.insert('1.0', value or '')
        if readonly:
            w.configure(state='disabled')
        self.widgets[key] = w; frame.pack(fill='both', expand=True, pady=6)
        if not readonly:
            w.bind('<KeyRelease>', lambda event: self.changed())
        return w

    def choice(self, parent, key, choices, value, callback=None):
        var = tk.StringVar(value=value)
        menu = tk.OptionMenu(parent, var, *choices, **({'command': callback} if callback else {}))
        menu.configure(bg=PAPER, fg=INK, bd=0, relief='flat', highlightthickness=0, padx=7, pady=5)
        self.widgets[key] = var; return menu

    def check(self, parent, key, label, value=False):
        var = tk.BooleanVar(value=value); self.widgets[key] = var
        tk.Checkbutton(parent, text=label, variable=var, bg=BG, fg=INK, selectcolor=PAPER).pack(side='left', padx=6)
        return var

    def value(self, key):
        w = self.widgets[key]
        return text_value(w) if isinstance(w, tk.Text) else w.get()

    def request(self, route, data=None, callback=None, failure=None):
        self.workers.submit(lambda: self.api.request(route, data), callback, failure)

    def action(self, name, data=None, callback=None, failure=None):
        self.request('v2/action', {'action': name, 'input': data or {}}, callback, failure)

    def refresh(self, callback=None):
        def received(v):
            self.meta = v; self.state = v['state']
            if callback: callback()
        self.request('state', callback=received)

    def goto(self, name, capture=True):
        if capture: self.capture()
        self.page = name; self.generation += 1
        for child in self.content.winfo_children(): child.destroy()
        self.widgets = {}; self.buttons = {}
        title = [self.tr(zh, en) for key, zh, en in PAGES if key == name][0]
        self.crumb.configure(text='Workspace / ' + title)
        for key, button in self.nav.items(): button.configure(bg='#d6e7d9' if key == name else SIDE)
        tk.Label(self.content, text=title, font=('DejaVu Sans', 22, 'bold'), bg=BG, fg=INK).pack(anchor='w', pady=(0, 12))
        getattr(self, 'page_' + name)()
        self.canvas.xview_moveto(0); self.canvas.yview_moveto(0)

    def switch_language(self):
        self.capture(); target = 'en' if self.lang != 'en' else 'zh'
        def done(v):
            self.lang = target; current = self.page; self.page = ''; self.shell(); self.goto(current)
        self.request('settings', {'language': target}, done)

    def changed(self):
        self.dirty = True; self.epoch += 1
        self.status.set(self.tr('草稿待保存', 'Drafts pending save'))

    def draft(self):
        return self.data.setdefault('codeDrafts', {}).setdefault(self.code_language, {})

    def capture(self):
        if self.page == 'code' and 'source' in self.widgets:
            d = self.draft(); d['code'] = self.value('source'); d['requirements'] = self.value('requirements')
            self.data['codeLanguage'] = self.code_language
        elif self.page == 'calculator' and 'expression' in self.widgets:
            c = self.data.setdefault('calculator', {}); c.update(expression=self.value('expression'), variables=self.value('variables'), angle=self.value('angle'))
        elif self.page == 'chat' and 'chat_input' in self.widgets:
            c = self.data.setdefault('chat', {}); c['text'] = self.value('chat_input'); c['session'] = self.session
        elif self.page == 'analysis' and self.analysis and 'analysis_question' in self.widgets:
            for saved in self.data.get('analyses') or []:
                if saved['id'] == self.analysis_id: saved['question'] = self.value('analysis_question')
        elif self.page == 'eda' and 'eda_input' in self.widgets:
            self.data.setdefault('eda', {}).setdefault(self.eda_id, {})['text'] = self.value('eda_input')

    def save_workspace(self, complete=None):
        self.capture()
        if self.saving:
            self.root.after(150, lambda: self.save_workspace(complete)); return
        self.saving = True; epoch = self.epoch
        payload = {'schema': 1, 'revision': self.workspace['revision'], 'data': copy.deepcopy(self.data)}
        def done(v):
            self.workspace['revision'] = v['revision']; self.saving = False; self.save_error = False
            if self.epoch == epoch: self.dirty = False
            if self.dirty:
                self.save_workspace(complete)
                return
            self.info(self.tr('已保存到本机', 'Saved locally'))
            if complete: complete()
        def failed(error):
            self.saving = False; self.save_error = True; self.error(error)
        self.request('workspace', payload, done, failed)

    def recovery(self):
        self.capture()
        if messagebox.askyesno(self.tr('草稿恢复', 'Draft recovery'), self.tr('先导出当前草稿，再重新加载本机版本？', 'Export current drafts, then reload the saved version?'), parent=self.root):
            export_text(self.root, pretty(self.data), 'icw-drafts-recovery.json')
            if not messagebox.askyesno('IC Workbench', self.tr('确认替换当前未保存草稿？', 'Replace current unsaved drafts?'), parent=self.root): return
            def got(w):
                self.workspace = w; self.data = workspace_defaults(w.get('data')); self.dirty = False; self.save_error = False; self.goto(self.page, False)
            self.request('workspace', callback=got)

    def page_chat(self):
        c = self.data.setdefault('chat', {})
        budget = self.engineering_data().get('nextBudget')
        if budget:
            row = self.row(); self.label(row, self.tr('模板任务：只读 · 最多 %d 轮 / %d 秒', 'Template: read-only · max %d rounds / %d s') % (budget['maxRounds'],budget['maxSeconds']))
            def clear_budget(): self.engineering_data().pop('nextBudget',None); self.changed(); self.goto('chat')
            self.button(row,'template_clear',self.tr('恢复普通对话','Use normal chat'),clear_budget).pack(side='right')
        row = self.row(); self.button(row, 'task_templates', self.tr('参数化任务模板', 'Task templates'), self.task_templates).pack(side='left', padx=5)
        self.button(row, 'new_chat', self.tr('新对话', 'New conversation'), self.new_chat).pack(side='left')
        self.button(row, 'scope', self.tr('选择项目 / 文件夹', 'Project / folder'), self.chat_scope).pack(side='left', padx=8)
        self.label(row, (c.get('folder') or c.get('project') or self.tr('通用问答', 'General questions')))
        row = self.row()
        sessions = [(x['id'], x.get('title', x['id'])) for x in self.state.get('sessions') or [] if not x.get('edaConnection')]
        self.session_options = dict((title + ' [' + sid[:6] + ']', sid) for sid, title in sessions)
        choices = [self.tr('历史对话', 'History')] + sorted(self.session_options)
        self.choice(row, 'session_select', choices, choices[0], self.select_session).pack(side='left')
        self.button(row, 'rename_chat', self.tr('重命名', 'Rename'), self.rename_chat).pack(side='left', padx=6)
        self.choice(row, 'linux_mode', ['off', 'confirm', 'auto'], 'off').pack(side='left', padx=10)
        self.check(row, 'allow_write', self.tr('允许平台记录修改', 'Allow platform record changes'))
        self.label(self.content, self.tr('Linux Agent：off 关闭 / confirm 逐项确认 / auto 本轮自主。命令使用当前账号权限。', 'Linux Agent: off / confirm each action / auto for this turn. Commands use your account permissions.'))
        self.button(self.content, 'chat_details', self.tr('查看原始回执', 'View original receipts'), self.chat_details).pack(anchor='e')
        self.editor(self.content, 'messages', '', 10, True)
        self.editor(self.content, 'progress', '', 5, True)
        self.editor(self.content, 'chat_input', c.get('text', ''), 4)
        row = self.row()
        self.button(row, 'send', self.tr('发送任务', 'Send task'), self.send_chat, True).pack(side='right')
        self.button(row, 'cancel_agent', self.tr('停止', 'Stop'), lambda: self.request('v2/cancel-agent', {'id': self.agent})).pack(side='right', padx=8)
        self.button(row, 'approve', self.tr('确认执行', 'Approve'), lambda: self.approve(True)).pack(side='left')
        self.button(row, 'decline', self.tr('拒绝', 'Decline'), lambda: self.approve(False)).pack(side='left', padx=6)
        if self.agent_snapshot: self.render_agent(self.agent_snapshot)
        self.load_history()

    def new_chat(self):
        if self.agent: raise ValueError(self.tr('请先停止或等待当前任务', 'Stop or finish the active task first'))
        self.session = ''; self.data.setdefault('chat', {})['session'] = ''; self.changed(); self.goto('chat')

    def select_session(self, name):
        if self.agent: return
        self.session = self.session_options.get(name, ''); self.data.setdefault('chat', {})['session'] = self.session
        self.changed(); self.load_history()

    def rename_chat(self):
        if not self.session: return
        self.form(self.tr('重命名', 'Rename'), [('title', '标题 / Title', '')], {},
                  lambda d: self.request('sessions', {'id': self.session, 'title': d['title']}, lambda v: self.refresh(lambda: self.goto('chat'))))

    def chat_scope(self):
        if self.agent: raise ValueError(self.tr('任务运行期间不能切换范围', 'Cannot change scope while a task is active'))
        items = [(p['id'], p['name']) for p in self.state.get('projects') or []]
        text = '\n'.join(p[0] + ' : ' + p[1] for p in items)
        self.form(self.tr('操作范围：切换后开始新对话', 'Scope: switching starts a new conversation'),
                  [('project', '项目 ID / Project ID (' + text + ')', ''), ('folder', '或文件夹 / Or folder', 'directory')], self.data.get('chat') or {}, self.set_scope)

    def set_scope(self, d):
        if d['project'] and d['folder']: raise ValueError('Select a project OR a folder')
        if d['project'] and not any(p['id'] == d['project'] for p in self.state.get('projects') or []): raise ValueError('Unknown project')
        if d['folder'] and not os.path.isdir(d['folder']): raise ValueError('Folder unavailable')
        c = self.data.setdefault('chat', {}); c.update(project=d['project'], folder=d['folder'], scope='project' if d['project'] else ('folder' if d['folder'] else 'general'), session='')
        self.session = ''; self.changed(); self.goto('chat')

    def send_chat(self):
        if self.agent: raise ValueError('A task is already running')
        text = self.value('chat_input'); mode = self.value('linux_mode'); allow = bool(self.value('allow_write'))
        if not text.strip(): return
        self.widgets['linux_mode'].set('off'); self.widgets['allow_write'].set(False)
        if mode == 'auto' and not messagebox.askyesno('Linux Agent', self.tr('本轮允许模型以你的 Linux 权限自主执行命令及文件操作，继续？', 'Allow autonomous commands and file operations with your Linux permissions for this turn?'), parent=self.root): return
        scope = self.data.get('chat') or {}
        def start(s):
            self.session = s['id']; self.data.setdefault('chat', {})['session'] = self.session
            self.changed()
            self.request('v2/agent', {'session': self.session, 'text': text, 'linuxMode': mode, 'allowWrite': allow, 'maxRounds': (self.engineering_data().get('nextBudget') or {}).get('maxRounds',12), 'maxSeconds': (self.engineering_data().get('nextBudget') or {}).get('maxSeconds',600), 'readOnly': bool(self.engineering_data().get('nextBudget')), 'templateID': (self.engineering_data().get('nextBudget') or {}).get('templateID',''), 'templateValues': (self.engineering_data().get('nextBudget') or {}).get('templateValues',{})}, self.agent_started)
        if self.session: start({'id': self.session})
        else: self.request('sessions', {'title': text[:70], 'project': scope.get('project', ''), 'folder': scope.get('folder', '')}, start)

    def agent_started(self, r):
        self.data.setdefault('chat', {})['text'] = ''; self.changed()
        if self.page == 'chat': set_text(self.widgets['chat_input'], '')
        self.engineering_data().pop('nextBudget', None); self.agent = r['id']; self.load_history(); self.poll_agent()

    def load_history(self):
        if not self.session or self.page != 'chat': return
        sid = self.session
        def loaded(s):
            if self.page != 'chat' or self.session != sid: return
            self.chat_raw = '\n\n'.join(m['role'].upper() + '\n' + m['content'] for m in s.get('messages') or [])
            set_text(self.widgets['messages'], readable_chat(self.chat_raw))
            if s.get('lastTask'):
                task = s['lastTask']; self.render_agent(task)
                if task['status'] in ('running', 'awaiting_approval') and not self.agent:
                    self.agent = task['id']; self.poll_agent()
        self.request('session?id=' + sid, callback=loaded)

    def poll_agent(self):
        if not self.agent: return
        identity = self.agent
        def got(r):
            if identity != self.agent: return
            self.agent_snapshot = r; self.render_agent(r)
            if r['status'] in ('running', 'awaiting_approval'):
                self.root.after(900, self.poll_agent)
            else:
                self.agent = ''; self.refresh(); self.load_history()
        self.request('v2/progress?id=' + identity, callback=got)

    def render_agent(self, r):
        if self.page != 'chat': return
        text = r['status'] + '\n' + '\n'.join(e['stage'] + ' · ' + (e['text'].split('\n')[0].split(' · ')[0] if e['stage'] in ('tool','receipt','invalid-tool-text','template-input') else e['text']) for e in (r.get('events') or [])[-12:])
        if r.get('pending'): text += '\n' + pretty(r['pending'])
        if r.get('terminal'): text += '\n' + pretty(r['terminal'])
        if r.get('error'): text += '\nERROR: ' + r['error']
        set_text(self.widgets['progress'], text[-20000:])
        for key in ('approve', 'decline'):
            if key in self.buttons: self.buttons[key].configure(state='normal' if r['status'] == 'awaiting_approval' else 'disabled')

    def approve(self, yes):
        if self.agent: self.request('v2/approve', {'id': self.agent, 'approve': yes})

    def form(self, title, specs, values, submit):
        dialog = tk.Toplevel(self.root); dialog.title(title); dialog.geometry('760x660'); dialog.configure(bg=BG)
        canvas = tk.Canvas(dialog, bg=BG, highlightthickness=0); scroll = tk.Scrollbar(dialog, command=canvas.yview)
        bottom = tk.Frame(dialog, bg=BG); bottom.pack(side='bottom', fill='x', padx=18, pady=12)
        scroll.pack(side='right', fill='y'); canvas.pack(fill='both', expand=True); canvas.configure(yscrollcommand=scroll.set)
        inner = tk.Frame(canvas, bg=BG); item = canvas.create_window(0, 0, window=inner, anchor='nw')
        inner.bind('<Configure>', lambda e: canvas.configure(scrollregion=canvas.bbox('all')))
        canvas.bind('<Configure>', lambda e: canvas.itemconfigure(item, width=e.width))
        fields = {}
        for key, label, kind in specs:
            box = tk.Frame(inner, bg=BG); box.pack(fill='x', padx=20, pady=5)
            tk.Label(box, text=label, bg=BG, fg=INK, anchor='w', wraplength=690).pack(anchor='w')
            value = values.get(key, '')
            if kind == 'bool':
                w = tk.BooleanVar(value=bool(value)); tk.Checkbutton(box, variable=w, bg=BG).pack(anchor='w')
            elif kind == 'multiline':
                w = tk.Text(box, height=4, bg=PAPER, relief='flat'); w.insert('1.0', value or ''); w.pack(fill='x')
            else:
                w = tk.Entry(box, bg=PAPER, relief='flat'); w.insert(0, value); w.pack(fill='x', ipady=7)
                if kind == 'directory':
                    def choose(entry=w):
                        value = filedialog.askdirectory(parent=dialog)
                        if value: entry.delete(0, 'end'); entry.insert(0, value)
                    tk.Button(box, text=self.tr('浏览…', 'Browse…'), command=choose).pack(anchor='e')
            fields[key] = w
        def save():
            try:
                result = dict((k, text_value(w) if isinstance(w, tk.Text) else (bool(w.get()) if isinstance(w, tk.BooleanVar) else w.get())) for k, w in fields.items())
                submit(result); dialog.destroy()
            except Exception as error: self.error(error)
        tk.Button(bottom, text=self.tr('保存 / 确认', 'Save / confirm'), bg=GREEN, fg='white', command=save, padx=20, pady=8).pack(side='right')
        tk.Button(bottom, text=self.tr('取消', 'Cancel'), command=dialog.destroy, padx=20, pady=8).pack(side='right', padx=8)
        dialog.transient(self.root); dialog.grab_set(); self.dialog_fields = fields; self.dialog_submit = save

    def page_projects(self):
        row = self.row()
        self.button(row, 'project_add', self.tr('添加项目', 'Add project'), lambda: self.project_form({}), True).pack(side='left')
        self.button(row, 'project_edit', self.tr('编辑选中项目', 'Edit selected'), lambda: self.project_form(self.project or {})).pack(side='left', padx=8)
        self.entry(row, 'query', '').pack(side='left', fill='x', expand=True, padx=10, ipady=7)
        self.button(row, 'search', self.tr('搜索全部项目', 'Search all projects'), lambda: self.search_files('')).pack(side='right')
        self.project_table = Table(self.content, [self.tr('项目', 'Project'), self.tr('路径', 'Path'), self.tr('标签', 'Tags'), self.tr('备注', 'Notes')], self.select_project, 4)
        self.project_table.pack(fill='x', pady=8)
        self.project_table.set(self.state.get('projects') or [], lambda p: [p['name'], p['root'], ', '.join(p.get('tags') or []), p.get('notes', '')])
        row = self.row(); self.label(row, self.tr('当前项目目录', 'Current project directory'))
        roots = ([self.project['root']] + (self.project.get('paths') or [])) if self.project else ['']
        self.choice(row, 'root', roots, self.file_root if self.file_root in roots else roots[0], self.change_root).pack(side='left', fill='x', expand=True)
        self.button(row, 'up', '↑', self.file_up).pack(side='left', padx=5)
        self.button(row, 'browse', self.tr('刷新', 'Refresh'), self.browse).pack(side='left', padx=5)
        self.button(row, 'next_files', self.tr('下一页', 'Next page'), self.next_files).pack(side='left', padx=5)
        self.file_table = Table(self.content, [self.tr('文件 / 目录', 'File / directory'), self.tr('备注', 'Notes'), self.tr('说明', 'Description')], height=7)
        self.file_table.pack(fill='both', expand=True)
        self.file_table.view.bind('<Double-1>', lambda e: self.safe(self.open_file))
        row = self.row()
        self.button(row, 'file_open', self.tr('打开选中项', 'Open selected'), self.open_file).pack(side='left')
        self.button(row, 'file_note', self.tr('备注 / 说明', 'Annotate'), self.annotate).pack(side='left', padx=8)
        self.button(row, 'search_more', self.tr('继续搜索', 'Continue search'), self.continue_search).pack(side='left', padx=8)
        self.editor(self.content, 'preview', '', 5, True)
        if self.project: self.browse()

    def project_form(self, p):
        values = dict(p); values['tags'] = ', '.join(p.get('tags') or []); values['paths'] = '\n'.join(p.get('paths') or [])
        def save(d):
            v = dict(p); v.update(d); v['tags'] = [s.strip() for s in d['tags'].split(',') if s.strip()]; v['paths'] = [s.strip() for s in d['paths'].splitlines() if s.strip()]
            self.action('save-project', v, lambda result: self.refresh(lambda: self.goto('projects')))
        self.form(self.tr('项目资料', 'Project details'), [('name', '名称 / Name', ''), ('root', '主目录 / Root', 'directory'), ('paths', '其他目录，每行一个 / Other directories', 'multiline'), ('tags', '标签，逗号分隔 / Tags', ''), ('notes', '备注 / Notes', 'multiline'), ('description', '说明 / Description', 'multiline')], values, save)

    def select_project(self, p):
        if not p or (self.project and self.project['id'] == p['id']): return
        self.project = p; self.file_root = p['root']; self.file_path = ''; self.file_offset = 0; self.goto('projects')

    def change_root(self, root):
        self.file_root = root; self.file_path = ''; self.file_offset = 0; self.browse()

    def file_request_guard(self, listing=False):
        # A page can stay mounted while its directory, query or preview changes.
        # A shared listing token prevents browse/search responses replacing each
        # other; a separate preview token also expires whenever that list changes.
        key = 'file_list_request' if listing else 'file_preview_request'
        token = (self.generation, uuid.uuid4().hex)
        setattr(self, key, token)
        if listing:
            self.file_preview_request = None; self.search_cursor = ''
            self.file_table.set([], lambda row: [])
            for name in ('next_files', 'search_more'):
                self.buttons[name].configure(state='disabled')
        set_text(self.widgets['preview'], '')
        return lambda: self.page == 'projects' and self.generation == token[0] and getattr(self, key, None) == token

    def browse(self):
        if not self.project: return
        scope = (self.project['id'], self.file_root, self.file_path, self.file_offset)
        selected = self.file_table.selected() if getattr(self, 'file_listing_scope', None) == scope else None
        current = self.file_request_guard(True)
        location = self.file_root + '/' + self.file_path
        def got(r):
            if not current(): return
            rows = r.get('entries') or []
            self.file_table.set(rows, lambda x: [('▸ ' if x.get('directory') else '') + x.get('name', ''), x.get('notes', ''), x.get('description', '')])
            self.file_listing_scope = scope
            if selected:
                for i, item in enumerate(rows):
                    if item.get('path') == selected.get('path'):
                        self.file_table.select(i); break
            self.info(location)
            self.buttons['next_files'].configure(state='normal' if r.get('more') else 'disabled')
        self.action('browse', {'project': self.project['id'], 'root': self.file_root, 'path': self.file_path, 'offset': self.file_offset}, got)

    def next_files(self): self.file_offset += 200; self.browse()
    def file_up(self): self.file_path = os.path.dirname(self.file_path); self.file_offset = 0; self.browse()

    def open_file(self):
        item = self.file_table.selected()
        if not item: return
        project = item.get('project') or (self.project or {}).get('id')
        root = item.get('root') or self.file_root
        if item.get('directory'):
            self.file_path = item['path']; self.file_offset = 0; self.browse(); return
        current = self.file_request_guard()
        self.action('browse', {'project': project, 'root': root, 'path': item['path']},
                    lambda r: set_text(self.widgets['preview'], r.get('content', pretty(r))) if current() else None)

    def annotate(self):
        item = self.file_table.selected()
        if not item or not self.project: return
        def save(d):
            d.update(project=item.get('project') or self.project['id'], root=item.get('root') or self.file_root, path=item['path'])
            self.action('note', d, lambda r: self.browse())
        self.form(item.get('path', ''), [('notes', '备注 / Notes', 'multiline'), ('description', '说明 / Description', 'multiline')], item, save)

    def search_files(self, cursor):
        query = self.value('query')
        if query != getattr(self, 'search_query', ''): cursor = ''
        self.search_query = query; self.file_listing_scope = None; current = self.file_request_guard(True)
        def got(r):
            if not current(): return
            self.search_cursor = r.get('cursor', '') if r.get('more') else ''
            rows = r.get('hits') or []
            self.file_table.set(rows, lambda x: [x.get('path', ''), x.get('line', ''), x.get('text', x.get('snippet', ''))])
            set_text(self.widgets['preview'], pretty(r)); self.buttons['search_more'].configure(state='normal' if self.search_cursor else 'disabled')
        self.action('search', {'query': self.search_query, 'cursor': cursor}, got)

    def continue_search(self): self.search_files(getattr(self, 'search_cursor', ''))

    def page_software(self):
        row = self.row()
        for key, zh, en, fn in [('software_add', '添加软件', 'Add software', lambda: self.software_form({})), ('software_edit', '编辑', 'Edit', lambda: self.software_form(self.software_table.selected() or {})), ('version_add', '添加 / 编辑版本', 'Add / edit version', self.software_version), ('software_run', '启动选中版本', 'Launch version', self.software_launch)]:
            self.button(row, key, self.tr(zh, en), fn).pack(side='left', padx=4)
        self.software_table = Table(self.content, ['Software', 'Command', 'Active version'], self.software_selected, 8); self.software_table.pack(fill='both', expand=True)
        self.software_table.set(self.state.get('software') or [], lambda s: [s['name'], s['command'], s['active']])
        self.version_row = self.row(); self.choice(self.version_row, 'version', [''], '').pack(side='left')
        self.editor(self.content, 'operation_log', '', 7, True)
        self.show_operations('software')

    def software_selected(self, s):
        if not s: return
        for w in self.version_row.winfo_children(): w.destroy()
        self.choice(self.version_row, 'version', [v['name'] for v in s['versions']], s['active'], self.set_software_version).pack(side='left')
        self.label(self.version_row, s['shell'])

    def set_software_version(self, version):
        s = self.software_table.selected()
        if not s: return
        v = copy.deepcopy(s); v['active'] = version
        self.action('save-software', v, lambda r: self.refresh())

    def software_form(self, s):
        def saved(d):
            d.update(id=s.get('id', ''), versions=s.get('versions') or [{'name': 'default', 'setup': '', 'toolPath': ''}], active=s.get('active') or 'default')
            self.action('save-software', d, lambda r: self.refresh(lambda: self.goto('software')))
        self.form('Software', [('name', '名称 / Name', ''), ('command', '启动语句 / Command', 'multiline'), ('shell', 'Shell: /bin/bash, /bin/csh …', '')], dict({'shell': '/bin/bash'}, **s), saved)

    def software_version(self):
        s = self.software_table.selected()
        if not s: return
        current = next((v for v in s['versions'] if v['name'] == self.value('version')), {})
        def saved(d):
            value = copy.deepcopy(s); value['versions'] = [v for v in value['versions'] if v['name'] != d['name']] + [d]; value['active'] = d['name']
            self.action('save-software', value, lambda r: self.refresh(lambda: self.goto('software')))
        self.form(self.tr('修改名称可添加新版本', 'Change name to add a version'), [('name', '版本名称 / Version', ''), ('setup', '环境脚本绝对路径 / Environment script', ''), ('toolPath', '工具 PATH / Tool PATH', '')], current, saved)

    def software_launch(self):
        s = self.software_table.selected()
        if s and messagebox.askyesno('Launch', s['command'], parent=self.root):
            self.action('launch-software', {'id': s['id'], 'version': self.value('version')}, self.operation_started)

    def page_plugins(self):
        row = self.row()
        for key, zh, en, fn in [('plugin_add', '添加插件', 'Add plugin', lambda: self.plugin_form({})), ('plugin_edit', '编辑', 'Edit', lambda: self.plugin_form(self.plugin_table.selected() or {})), ('plugin_import', '批量导入 JSON', 'Import JSON', self.plugin_import), ('plugin_run', '执行', 'Run', self.plugin_run), ('plugin_guide', '使用说明', 'Guide', self.plugin_guide)]:
            self.button(row, key, self.tr(zh, en), fn).pack(side='left', padx=4)
        self.plugin_table = Table(self.content, ['Name', 'Version', 'Enabled', 'Description'], height=8); self.plugin_table.pack(fill='both', expand=True)
        self.plugin_table.set(self.state.get('plugins') or [], lambda p: [p['name'], p['version'], str(p['enabled']), p['description']])
        self.editor(self.content, 'operation_log', '', 8, True); self.show_operations('plugin')

    def plugin_form(self, p):
        v = dict(p); v.update(executable=p.get('command', {}).get('executable', ''), args=pretty(p.get('command', {}).get('args') or []), timeoutSeconds=str(p.get('timeoutSeconds', 60)))
        def save(d):
            d['id'] = p.get('id') or uuid.uuid4().hex; d['timeoutSeconds'] = int(d['timeoutSeconds']); d['command'] = {'executable': d.pop('executable'), 'args': json.loads(d.pop('args'))}
            self.action('install-plugins', {'plugins': [d]}, lambda r: self.refresh(lambda: self.goto('plugins')))
        self.form('Plugin', [('name', '名称 / Name', ''), ('version', '版本 / Version', ''), ('description', '说明 / Description', 'multiline'), ('executable', '程序绝对路径 / Executable', ''), ('args', '参数 JSON 数组 / Args', 'multiline'), ('timeoutSeconds', '超时秒数 / Timeout seconds', ''), ('enabled', '启用 / Enabled', 'bool')], v, save)

    def plugin_import(self):
        path = filedialog.askopenfilename(parent=self.root, filetypes=[('JSON', '*.json')])
        if path:
            value = json.loads(read_text(path, 1048576))
            self.action('install-plugins', value if isinstance(value, dict) else {'plugins': value}, lambda r: self.refresh(lambda: self.goto('plugins')))

    def plugin_run(self):
        p = self.plugin_table.selected()
        if p and messagebox.askyesno('Run plugin', pretty(p['command']), parent=self.root): self.action('run-plugin', {'id': p['id']}, self.operation_started)

    def plugin_guide(self):
        messagebox.showinfo('Plugins', self.tr('填写插件程序绝对路径及参数即可添加；批量 JSON 使用 {"plugins":[...]}。程序以当前用户执行。脚本批量添加方法见安装目录 install_plugins.py --help 与 DEPLOYMENT.md。添加不会立即执行。', 'Register an absolute executable and argument list. Bulk JSON uses {"plugins":[...]}. Execution uses your account. See install_plugins.py --help and DEPLOYMENT.md in the installation. Registration does not execute a plugin.'), parent=self.root)

    def operation_started(self, r):
        self.last_operation = r['id']; self.poll_operation(r['id'])

    def poll_operation(self, identity):
        def got(r):
            if 'operation_log' in self.widgets: set_text(self.widgets['operation_log'], r.get('status', '') + '\n' + r.get('log', '') + '\n' + r.get('error', ''))
            if r['status'] in ('queued', 'running'): self.root.after(1000, lambda: self.poll_operation(identity))
            else: self.refresh()
        self.action('operation', {'id': identity}, got)

    def show_operations(self, kind):
        row = self.row()
        self.button(row, 'operation_cancel', self.tr('停止当前操作', 'Stop current operation'), lambda: self.action('cancel-operation', {'id': getattr(self, 'last_operation', '')})).pack(side='left')
        ops = [o for o in self.state.get('operations') or [] if o.get('kind') == kind]
        set_text(self.widgets['operation_log'], '\n'.join(o.get('name', '') + ' · ' + o['status'] + '\n' + o.get('log', '')[-3000:] for o in ops[-5:]))

    def page_monitor(self):
        self.resource_labels = {}
        row = self.row()
        for key in ['cpu', 'memory', 'storage', 'gpu']:
            w = tk.Label(row, text=key.upper() + ' —', bg=SIDE, fg=GREEN, font=('DejaVu Sans', 16), padx=20, pady=18)
            w.pack(side='left', fill='x', expand=True, padx=5); self.resource_labels[key] = w
        self.process_table = Table(self.content, ['PID', 'Process', 'CPU %', 'Memory MiB', 'Simulation'], height=13); self.process_table.pack(fill='both', expand=True, pady=12)
        row = self.row()
        self.button(row, 'terminate', self.tr('关闭选中任务', 'Stop selected task'), self.terminate).pack(side='left')
        self.button(row, 'monitor_refresh', self.tr('刷新', 'Refresh'), self.poll_monitor).pack(side='left', padx=8)
        self.editor(self.content, 'monitor_notes', '', 3, True); self.poll_monitor()

    def poll_monitor(self):
        if self.polling: return
        self.polling = True
        def got(r):
            self.polling = False; self.alarm = '; '.join(r.get('alerts') or [])
            if self.alarm: self.status.set(self.alarm)
            if self.page != 'monitor': return
            for key in ['cpu', 'memory', 'storage']:
                value = r.get(key); self.resource_labels[key].configure(text=key.upper() + (' —' if value is None else ' %.1f%%' % value))
            gpu = r.get('gpus') or []; self.resource_labels['gpu'].configure(text='GPU ' + ('%.1f%%' % max(g['usage'] for g in gpu) if gpu else 'N/A'))
            rows = sorted(r.get('processes') or [], key=lambda p: (p['cpu'], p['memoryBytes']), reverse=True)
            self.process_table.set(rows, lambda p: [p['pid'], p['name'], '%.1f' % p['cpu'], '%.1f' % (p['memoryBytes']/1048576.0), str(p['simulation'])])
            set_text(self.widgets['monitor_notes'], '\n'.join((r.get('alerts') or []) + (r.get('notes') or [])))
        def failure(e): self.polling = False; self.info(str(e))
        self.action('monitor', callback=got, failure=failure)

    def terminate(self):
        p = self.process_table.selected()
        if not p: return
        if not p['own']: raise ValueError('Only processes owned by your UID can be stopped')
        if messagebox.askyesno('SIGTERM', str(p['pid']) + ' ' + p['name'], parent=self.root):
            self.action('terminate', {'pid': p['pid'], 'start': p['start']}, lambda r: self.poll_monitor())

    def page_analysis(self):
        self.button(self.content, 'metric_compare', self.tr('指标比较 · 单位与工况', 'Metrics · units & conditions'), self.metric_compare).pack(anchor='e')
        row = self.row()
        self.button(row, 'analysis_import', self.tr('导入文件', 'Import file'), self.import_analysis, True).pack(side='left')
        self.analysis_options = dict((a['name'] + ' [' + a['id'][-8:] + ']', a['id']) for a in self.data.get('analyses') or [])
        choices = [self.tr('历史记录', 'History')] + sorted(self.analysis_options)
        self.choice(row, 'analysis_history', choices, choices[0], self.select_analysis).pack(side='left', padx=12)
        self.button(row, 'analysis_export', self.tr('导出分析', 'Export analysis'), lambda: export_text(self.root, self.value('analysis_answer'), 'analysis.txt')).pack(side='right')
        self.editor(self.content, 'analysis_text', self.analysis.get('text', '') if self.analysis else '', 7, True)
        row = self.row()
        for key, label, fn in [('zoom_in', '+', lambda: self.zoom(1.5)), ('zoom_out', '−', lambda: self.zoom(1/1.5)), ('pan_left', '←', lambda: self.pan(-.15)), ('pan_right', '→', lambda: self.pan(.15))]:
            self.button(row, key, label, fn).pack(side='left', padx=4)
        self.wave_canvas = tk.Canvas(self.content, bg=PAPER, height=170, highlightthickness=0); self.wave_canvas.pack(fill='x', pady=5)
        self.wave_canvas.bind('<Configure>', lambda e: self.draw_wave())
        self.wave_canvas.bind('<MouseWheel>', lambda e: self.zoom(1.3 if e.delta > 0 else 1/1.3))
        self.editor(self.content, 'analysis_question', '', 2)
        self.button(self.content, 'analysis_ai', self.tr('AI 分析当前文件', 'Analyze this file with AI'), self.analyze_ai, True).pack(anchor='e')
        self.editor(self.content, 'analysis_answer', '', 6, True)
        if self.analysis_id:
            saved = next((a for a in self.data.get('analyses') or [] if a['id'] == self.analysis_id), None)
            if saved: self.restore_analysis(saved)

    def import_analysis(self, path=None):
        path = path or filedialog.askopenfilename(parent=self.root)
        if not path: return
        text = read_text(path, 2*1048576); name = os.path.basename(path)
        def got(r):
            self.analysis = r; self.analysis_id = 'analysis-' + str(int(time.time()*1000000)); self.data['analysisID'] = self.analysis_id
            saved = {'id': self.analysis_id, 'name': name, 'text': text, 'question': '', 'answer': '', 'limited': False}
            self.data.setdefault('analyses', []).append(saved); self.changed()
            if self.page == 'analysis': self.goto('analysis')
        self.action('analysis', {'name': name, 'content': text}, got)

    def select_analysis(self, name):
        self.capture(); identity = self.analysis_options.get(name)
        saved = next((a for a in self.data.get('analyses') or [] if a['id'] == identity), None)
        if saved: self.restore_analysis(saved)

    def restore_analysis(self, saved):
        self.analysis_id = saved['id']; self.data['analysisID'] = self.analysis_id
        identity = self.analysis_id
        set_text(self.widgets['analysis_text'], saved['text']); set_text(self.widgets['analysis_question'], saved.get('question', '')); set_text(self.widgets['analysis_answer'], saved.get('answer', ''))
        def got(r):
            if identity != self.analysis_id: return
            self.analysis = r; self.wave_zoom = 1; self.wave_pan = 0
            if self.page == 'analysis': self.draw_wave()
        self.action('analysis', {'name': saved['name'], 'content': saved['text']}, got)

    def analyze_ai(self):
        if not self.analysis: return
        self.capture(); saved = next(a for a in self.data['analyses'] if a['id'] == self.analysis_id)
        identity = self.analysis_id
        self.info(self.tr('AI 正在分析…', 'AI is analyzing…'))
        def got(r):
            saved['answer'] = r['content']; self.changed()
            if self.page == 'analysis' and self.analysis_id == identity: set_text(self.widgets['analysis_answer'], r['content'])
        self.action('analyze-ai', {'name': saved['name'], 'content': saved['text'], 'question': saved.get('question', ''), 'limited': saved.get('limited', False)}, got)

    def zoom(self, factor): self.wave_zoom = max(1.0, min(1000.0, self.wave_zoom*factor)); self.wave_pan = min(self.wave_pan, 1-1/self.wave_zoom); self.draw_wave()
    def pan(self, amount): self.wave_pan = max(0, min(1-1/self.wave_zoom, self.wave_pan+amount/self.wave_zoom)); self.draw_wave()

    def draw_wave(self):
        if self.page != 'analysis': return
        c = self.wave_canvas; c.delete('all'); signals = (self.analysis or {}).get('signals') or []
        if not signals:
            c.create_text(18, 30, anchor='w', fill=MUTED, text=self.tr('导入 CSV / VCD 可查看波形并缩放', 'Import CSV / VCD to view and zoom waveforms')); return
        all_points = [p for s in signals for p in s['points']]
        lo = min(p['t'] for p in all_points); hi = max(p['t'] for p in all_points); span = hi-lo or 1
        start = lo+span*self.wave_pan; end = start+span/self.wave_zoom
        width = max(c.winfo_width()-170, 100); height = max(c.winfo_height(), 170); step = height/float(min(len(signals), 6))
        for i, s in enumerate(signals[:6]):
            points = [p for p in s['points'] if p.get('v') is not None]; values = [p['v'] for p in points] or [0, 1]
            bottom, top = min(values), max(values); scale = top-bottom or 1
            c.create_text(8, i*step+16, anchor='w', text=s['name'][:20], fill=GREEN)
            coords = []; previous = None
            for p in points:
                if p['t'] < start: previous = p; continue
                if p['t'] > end:
                    if previous and s.get('digital'): coords.extend([140+width, i*step+step-8-(previous['v']-bottom)/scale*(step-25)])
                    break
                x = 140+(p['t']-start)/(end-start)*width; y = i*step+step-8-(p['v']-bottom)/scale*(step-25)
                if previous and s.get('digital'):
                    old_y = i*step+step-8-(previous['v']-bottom)/scale*(step-25)
                    if not coords: coords.extend([140, old_y])
                    coords.extend([x, old_y])
                coords.extend([x, y]); previous = p
            if len(coords) >= 4: c.create_line(*coords, **{'fill': GREEN, 'width': 2})
        c.create_text(width+140, 8, anchor='ne', text='%.4g … %.4g %s' % (start, end, self.analysis.get('timescale', '')), fill=MUTED)

    def page_calculator(self):
        value = self.data.get('calculator') or {}; self.label(self.content, self.tr('表达式，例如 R*C、sqrt(x*x+y*y)、sin(pi/2)', 'Expression, e.g. R*C, sqrt(x*x+y*y), sin(pi/2)'))
        self.entry(self.content, 'expression', value.get('expression', 'R*C')).pack(fill='x', ipady=10)
        self.widgets['expression'].bind('<KeyRelease>', lambda e: self.changed())
        self.label(self.content, self.tr('变量，每行一项；支持工程后缀', 'Variables, one per line; engineering suffixes supported'))
        self.editor(self.content, 'variables', value.get('variables', 'R=1k\nC=1n'), 8)
        row = self.row(); self.choice(row, 'angle', ['rad', 'deg'], value.get('angle', 'rad'), lambda v: self.changed()).pack(side='left')
        self.button(row, 'calculate', self.tr('计算', 'Calculate'), self.calculate, True).pack(side='right')
        self.editor(self.content, 'calc_result', pretty(value.get('result', {})), 8, True)

    def calculate(self):
        self.capture(); value = self.data['calculator']
        def got(r):
            value['result'] = r; self.changed()
            if self.page == 'calculator': set_text(self.widgets['calc_result'], pretty(r))
        self.action('calculate', dict((k, value[k]) for k in ('expression', 'variables', 'angle')), got)

    def page_code(self):
        row = self.row(); self.button(row, 'change_sets', self.tr('AI 多文件修改集', 'AI multi-file changes'), self.change_sets).pack(side='left')
        self.label(row, self.tr('先生成候选，再逐文件审阅、采用与撤销', 'Propose, review, apply and undo each file'))
        d = self.draft(); row = self.row()
        ids = [v['id'] for v in self.meta['codeLanguages']]
        self.choice(row, 'code_language', ids, self.code_language, self.change_code_language).pack(side='left')
        self.button(row, 'code_import', self.tr('导入代码', 'Import source'), self.import_code).pack(side='left', padx=5)
        self.button(row, 'code_export', self.tr('导出代码', 'Export source'), lambda: export_text(self.root, self.value('source'), d.get('name') or 'design'+self.code_extension())).pack(side='left', padx=5)
        self.button(row, 'code_tools', self.tr('检查 / 格式工具', 'Checker / formatter'), self.code_tools).pack(side='left', padx=5)
        self.button(row, 'code_history', self.tr('历史', 'History'), self.code_history).pack(side='left', padx=5)
        pane = tk.PanedWindow(self.content, orient='horizontal', bg=BG, sashwidth=10, bd=0); pane.pack(fill='both', expand=True)
        left = tk.Frame(pane, bg=BG); right = tk.Frame(pane, bg=BG); pane.add(left, minsize=220); pane.add(right, minsize=300)
        self.label(left, self.tr('需求与约束', 'Requirements & constraints')); self.editor(left, 'requirements', d.get('requirements', ''), 8)
        self.label(left, self.tr('AI 候选 / 修改说明（先审阅再采用）', 'AI proposal / summary (review before applying)'))
        self.editor(left, 'proposal', d.get('proposal', ''), 8, True)
        self.label(right, self.tr('原文 / 当前代码', 'Original / current source')); self.editor(right, 'source', d.get('code', ''), 20)
        self.widgets['source'].configure(wrap='none', font=('DejaVu Sans Mono', 11))
        row = self.row()
        buttons = [('code_generate', 'AI 生成', 'Generate', lambda: self.code_ai(False)), ('code_improve', 'AI 修改', 'Improve', lambda: self.code_ai(True)), ('code_apply', '采用候选', 'Apply proposal', self.apply_code), ('code_undo', '撤销', 'Undo', self.undo_code), ('code_check', '检查', 'Check', lambda: self.code_workflow('check')), ('code_repair', '限次修复', 'Repair', lambda: self.code_workflow('repair')), ('code_format', '预览整份排版', 'Preview file layout', lambda: self.code_workflow('format')), ('code_format_selection', '预览选中行', 'Preview selected lines', lambda: self.code_workflow('selection')), ('code_stop', '停止', 'Stop', lambda: self.action('cancel-operation', {'id': self.code_run}))]
        for i, (key, zh, en, fn) in enumerate(buttons):
            if i == 4: row = self.row()
            self.button(row, key, self.tr(zh, en), fn).pack(side='left', padx=3)
        self.label(self.content, self.tr('排版只改缩进与空白，不验证逻辑；结果先显示候选，采用后才替换草稿。', 'Formatting changes layout, not logic. Review the proposal before applying to the draft.'))
        self.button(self.content, 'code_diff', self.tr('查看修改差异', 'View changes'), self.code_diff).pack(anchor='e')
        self.editor(self.content, 'code_log', d.get('summary', ''), 4, True)
        self.diagnostic_table = Table(self.content, ['Line', 'Column', 'Diagnostic'], height=3); self.diagnostic_table.pack(fill='x')
        self.diagnostic_table.view.bind('<Double-1>', lambda e: self.code_jump())

    def code_diff(self):
        self.capture(); d = self.draft()
        w = self.engineering_window(self.tr('修改差异：− 原文 / ＋ 候选', 'Changes: − original / + proposal'))
        self.engineering_text(w, diff_text(d.get('base', d.get('code', '')), d.get('proposal', d.get('code', ''))), 24, True)

    def code_extension(self): return next(v['extension'] for v in self.meta['codeLanguages'] if v['id'] == self.code_language)

    def change_code_language(self, language):
        self.capture(); self.code_language = language; self.data['codeLanguage'] = language; self.changed(); self.goto('code', False)

    def archive_code(self):
        if self.draft().get('code'):
            self.data.setdefault('codeArchive', []).append({'id': str(int(time.time()*1000000)), 'language': self.code_language, 'draft': copy.deepcopy(self.draft())})

    def import_code(self, path=None):
        path = path or filedialog.askopenfilename(parent=self.root)
        if not path: return
        source = read_text(path, 1048576); self.capture(); self.archive_code()
        extension = os.path.splitext(path)[1]
        language = next((v['id'] for v in self.meta['codeLanguages'] if v['extension'] == extension), self.code_language)
        self.code_language = language; self.draft().update(code=source, original=source, name=os.path.basename(path), proposal='', base='', undo='')
        self.changed(); self.goto('code', False)

    def code_ai(self, improve):
        self.capture(); d = self.draft(); base = d.get('code', ''); language = self.code_language
        payload = {'language': language, 'requirements': d.get('requirements', '')}
        if improve: payload['code'] = base
        self.info(self.tr('AI 正在生成候选，原文保持不变…', 'AI is preparing a proposal; original source is preserved…'))
        def got(r):
            d.update(base=base, proposal=r['code'], summary=r.get('summary') or self.tr('AI 候选尚未编译或仿真验证', 'AI proposal has not been compiled or simulated'))
            self.changed()
            if self.page == 'code' and self.code_language == language:
                set_text(self.widgets['proposal'], d['proposal']); set_text(self.widgets['code_log'], d['summary'])
        self.action('improve-code' if improve else 'generate-code', payload, got)

    def apply_code(self):
        self.capture(); d = self.draft()
        if not d.get('proposal'): return
        if d.get('base') != d.get('code'): raise ValueError(self.tr('原文已变更，请重新生成候选', 'Source changed; request a fresh proposal'))
        self.archive_code(); d['undo'] = d.get('code', ''); d['code'] = d.pop('proposal'); d['base'] = ''
        self.changed(); self.goto('code', False)

    def undo_code(self):
        self.capture(); d = self.draft()
        if 'undo' in d: d['code'] = d.pop('undo'); self.changed(); self.goto('code', False)

    def code_history(self):
        records = self.data.get('codeArchive') or []
        if not records: return
        text = '\n'.join(str(i)+' · '+r['language']+' · '+r['draft'].get('name', '') for i, r in enumerate(records))
        def restore(value):
            saved = copy.deepcopy(records[int(value['index'])]); self.capture(); self.archive_code()
            self.code_language = saved['language']; self.data.setdefault('codeDrafts', {})[self.code_language] = saved['draft']; self.changed(); self.goto('code', False)
        self.form(self.tr('恢复历史草稿', 'Restore draft history'), [('index', text, '')], {'index': str(len(records)-1)}, restore)

    def code_tools(self):
        def loaded(r):
            p = r['preferences']; compiler = (p.get('compilers') or {}).get(self.code_language, {}); formatter = (p.get('formatters') or {}).get(self.code_language, {})
            def save(v):
                p.setdefault('compilers', {})[self.code_language] = {'executable': v['checker'], 'args': json.loads(v['check_args'])}
                p.setdefault('formatters', {})[self.code_language] = {'executable': v['formatter'], 'args': json.loads(v['format_args'])}
                self.action('save-preferences', p, lambda x: self.info('Saved'))
            self.form('Code tools', [('checker', '检查器绝对路径 / Checker executable', ''), ('check_args', '参数 JSON，使用 {file} / Args', 'multiline'), ('formatter', '格式化器 / Formatter executable', ''), ('format_args', '参数 JSON / Args', 'multiline')], {'checker': compiler.get('executable', ''), 'check_args': pretty(compiler.get('args') or []), 'formatter': formatter.get('executable', ''), 'format_args': pretty(formatter.get('args') or [])}, save)
        self.action('preferences', callback=loaded)

    def code_workflow(self, mode):
        if self.code_run: raise ValueError('A code workflow is already active')
        self.capture(); d = self.draft(); payload = {'language': self.code_language, 'code': d.get('code', ''), 'requirements': d.get('requirements', ''), 'mode': mode, 'maxRepairs': 2}
        if mode == 'selection':
            payload['mode'] = 'format'
            try:
                first, last = self.widgets['source'].index('sel.first'), self.widgets['source'].index('sel.last -1c')
                payload.update(startLine=int(first.split('.')[0]), endLine=int(last.split('.')[0]))
            except tk.TclError:
                raise ValueError(self.tr('请先在原文代码框选中要排版的行', 'Select source lines before formatting'))
        def started(r):
            self.code_run = r['id']; d['run'] = r['id']; self.changed(); self.poll_code(d, self.code_language, r['id'])
        self.action('code-workflow', payload, started)

    def poll_code(self, draft, language, identity):
        def got(r):
            result = r.get('codeResult') or {}; draft['summary'] = r['status']+'\n'+r.get('log', '')+'\n'+r.get('error', '')
            if self.page == 'code' and self.code_language == language:
                set_text(self.widgets['code_log'], draft['summary'])
                checks = result.get('checks') or []; diagnostics = checks[-1].get('diagnostics') or [] if checks else []
                self.diagnostic_table.set(diagnostics, lambda v: [v['line'], v['column'], v['message']])
            if r['status'] in ('queued', 'running'): self.root.after(1000, lambda: self.poll_code(draft, language, identity))
            else:
                self.code_run = ''; draft['receivedRun'] = identity
                if result.get('code') and result['mode'] != 'check':
                    draft.update(base=result['base'], proposal=result['code'])
                    if self.page == 'code' and self.code_language == language: set_text(self.widgets['proposal'], draft['proposal'])
                self.changed()
        self.action('operation', {'id': identity}, got)

    def code_jump(self):
        d = self.diagnostic_table.selected()
        if d and d.get('currentFile'):
            position = '%d.%d' % (d['line'], max(0, d['column']-1)); self.widgets['source'].mark_set('insert', position); self.widgets['source'].see(position); self.widgets['source'].focus_set()

    def page_eda(self):
        rows = [{'id': 'builtin', 'name': 'Cadence bridge', 'transport': 'builtin'}] + (self.state.get('mcpConnections') or [])
        row = self.row()
        self.eda_choices = dict((r['name']+' ['+r['id']+']', r['id']) for r in rows)
        names = sorted(self.eda_choices); selected = next((n for n in names if self.eda_choices[n] == self.eda_id), names[0])
        self.choice(row, 'eda_connection', names, selected, self.select_eda).pack(side='left')
        for key, zh, en, fn in [('mcp_add', '添加', 'Add', lambda: self.mcp_form({})), ('mcp_edit', '编辑', 'Edit', lambda: self.mcp_form(next((r for r in rows if r['id'] == self.eda_id and r['id'] != 'builtin'), {}))), ('mcp_discover', '发现工具', 'Discover tools', self.discover_mcp)]:
            self.button(row, key, self.tr(zh, en), fn).pack(side='left', padx=5)
        row = self.row()
        self.button(row, 'mcp_disconnect', self.tr('断开连接', 'Disconnect'), lambda: self.action('disconnect-mcp', {'id': self.eda_id}, lambda r: self.info(self.tr('会话已断开', 'Session disconnected')))).pack(side='left')
        self.button(row, 'mcp_remove', self.tr('移除配置', 'Remove connection'), self.remove_mcp).pack(side='left', padx=5)
        self.button(row, 'cadence_config', self.tr('Cadence 工程配置', 'Cadence project setup'), self.cadence_form).pack(side='left', padx=5)
        self.button(row, 'mcp_guide', self.tr('接入说明', 'Connection guide'), lambda: messagebox.showinfo('MCP', self.tr('先部署软件对应的 MCP 服务，再添加 stdio 命令或 HTTP 地址。发现工具后填写允许列表。AI 只调用选中连接允许的工具。MCP 回执不等于异步仿真完成；需查询任务状态。断开不会保证停止远端仿真。详见安装目录 docs/EDA_MCP.md 与 EDA_ADAPTER.md。', 'Deploy a software-specific MCP server, register its stdio command or HTTP URL, discover tools and configure the allowlist. AI uses only allowed tools on the selected connection. An MCP receipt is not completion of asynchronous simulations: poll task status. Disconnect does not guarantee cancellation of remote jobs. See docs/EDA_MCP.md and EDA_ADAPTER.md in the installation.'), parent=self.root)).pack(side='left', padx=5)
        self.mcp_table = Table(self.content, ['Tool', 'Description'], height=4); self.mcp_table.pack(fill='x')
        row = self.row(); self.entry(row, 'mcp_args', '{}').pack(side='left', fill='x', expand=True, ipady=7)
        self.button(row, 'mcp_call', self.tr('调用工具', 'Call tool'), self.call_mcp).pack(side='right', padx=7)
        self.editor(self.content, 'eda_history', '', 6, True)
        draft = self.data.setdefault('eda', {}).setdefault(self.eda_id, {})
        self.editor(self.content, 'eda_input', draft.get('text', ''), 3)
        row = self.row()
        self.button(row, 'eda_send', self.tr('AI 执行 EDA 任务', 'AI EDA task'), self.send_eda, True).pack(side='right')
        self.button(row, 'eda_stop', self.tr('停止任务', 'Stop task'), lambda: self.request('v2/cancel-agent', {'id': self.eda_run})).pack(side='right', padx=8)
        self.editor(self.content, 'operation_log', '', 5, True)
        if draft.get('session'):
            self.request('session?id='+draft['session'], callback=lambda s: set_text(self.widgets['eda_history'], '\n\n'.join(m['role']+'\n'+m['content'] for m in s.get('messages') or [])) if self.page == 'eda' else None)

    def select_eda(self, name):
        self.capture(); self.eda_id = self.eda_choices[name]; self.goto('eda')

    def remove_mcp(self):
        if self.eda_id == 'builtin': return
        if messagebox.askyesno('MCP', self.tr('移除该连接配置？已提交的远端任务不受影响。', 'Remove this connection configuration? Submitted remote jobs are unaffected.'), parent=self.root):
            def removed(r):
                self.eda_id = 'builtin'; self.refresh(lambda: self.goto('eda'))
            self.action('remove-mcp', {'id': self.eda_id}, removed)

    def cadence_form(self):
        projects = self.state.get('projects') or []
        if not projects: raise ValueError(self.tr('请先登记项目路径', 'Register a project first'))
        def chosen(v):
            p = next((copy.deepcopy(p) for p in projects if p['id'] == v['id']), None)
            if p is None: raise ValueError('Unknown project ID')
            def save(d):
                p.update(d); self.action('save-project', p, lambda r: self.refresh())
            self.form('Cadence · '+p['name'], [('cadenceConfig', '适配器 JSON 路径 / Adapter JSON path (EDA_ADAPTER.md)', ''), ('environment', '已登记环境 ID / Registered environment ID (optional)', '')], p, save)
        self.form('Cadence', [('id', '\n'.join(p['id']+' : '+p['name'] for p in projects), '')], {'id': projects[0]['id']}, chosen)

    def mcp_form(self, c):
        value = dict(c); value.update(executable=c.get('command', {}).get('executable', ''), args=pretty(c.get('command', {}).get('args') or []), allowedTools='\n'.join(c.get('allowedTools') or []), timeoutSeconds=str(c.get('timeoutSeconds', 60)), transport=c.get('transport', 'stdio'))
        def save(d):
            d.update(id=c.get('id', ''), command={'executable': d.pop('executable'), 'args': json.loads(d.pop('args'))})
            d['allowedTools'] = [s.strip() for s in d['allowedTools'].splitlines() if s.strip()]; d['timeoutSeconds'] = int(d['timeoutSeconds'])
            self.action('save-mcp', d, lambda r: self.refresh(lambda: self.goto('eda')))
        self.form('MCP', [('name', '名称 / Name', ''), ('transport', 'stdio / http', ''), ('executable', '程序绝对路径 / Executable', ''), ('args', '参数 JSON / Args', 'multiline'), ('directory', '工作目录 / Directory', 'directory'), ('url', 'HTTP URL', ''), ('tokenEnv', '令牌环境变量名称 / Token environment variable', ''), ('allowedTools', '允许的工具，每行一个 / Allowed tools', 'multiline'), ('timeoutSeconds', '超时秒数 / Timeout', '')], value, save)

    def discover_mcp(self):
        identity = self.eda_id
        def got(r):
            if self.page != 'eda' or identity != self.eda_id: return
            rows = r.get('tools', []) if isinstance(r, dict) else r
            self.mcp_table.set(rows, lambda t: [t['name'], t.get('description', '')]); self.info(self.tr('MCP 已连接', 'MCP connected'))
        self.action('mcp-tools', {'id': identity}, got)

    def call_mcp(self):
        tool = self.mcp_table.selected()
        if tool and messagebox.askyesno('MCP', tool['name']+'\n'+self.value('mcp_args'), parent=self.root):
            self.action('mcp-call', {'id': self.eda_id, 'tool': tool['name'], 'arguments': json.loads(self.value('mcp_args'))}, self.operation_started)

    def send_eda(self):
        if self.eda_run: raise ValueError(self.tr('请先停止或等待当前 EDA 任务', 'Stop or finish the active EDA task first'))
        self.capture(); identity = self.eda_id; draft = self.data.setdefault('eda', {}).setdefault(identity, {})
        def got(r):
            draft.update(session=r['session'], text=''); self.eda_run = r['id']; self.changed()
            if self.page == 'eda': set_text(self.widgets['eda_input'], '')
            self.poll_eda(identity, r['id'])
        self.action('eda-agent', {'id': identity, 'session': draft.get('session', ''), 'text': self.value('eda_input')}, got)

    def poll_eda(self, connection, identity):
        def got(r):
            if self.page == 'eda' and self.eda_id == connection:
                set_text(self.widgets['operation_log'], r['status']+'\n'+'\n'.join(e['text'] for e in (r.get('events') or [])[-15:])+'\n'+r.get('error', ''))
                if r.get('content'): set_text(self.widgets['eda_history'], r['content'])
            if r['status'] in ('running', 'awaiting_approval'): self.root.after(900, lambda: self.poll_eda(connection, identity))
            else: self.eda_run = ''; self.refresh()
        self.request('v2/progress?id='+identity, callback=got)

    def page_settings(self):
        p = self.preferences['preferences']; grid = tk.Frame(self.content, bg=BG); grid.pack(fill='x'); grid.columnconfigure(0, weight=1); grid.columnconfigure(1, weight=1)
        fields = [('modelUrl', '完整模型接口 / Full model URL'), ('model', '模型标识 / Model ID'), ('keyEnv', '密钥环境变量 / Key environment variable'), ('shell', 'Linux Shell'), ('workspace', '默认工作目录 / Working directory'), ('setup', '环境脚本路径 / Environment script'), ('threshold', '资源告警阈值 % / Alert threshold'), ('pollSeconds', '监测间隔秒 / Poll seconds')]
        for i, (key, label) in enumerate(fields):
            box = tk.Frame(grid, bg=BG); box.grid(row=i//2, column=i%2, sticky='ew', padx=8, pady=4)
            self.label(box, label); self.entry(box, 'setting_'+key, p.get(key, '')).pack(fill='x', ipady=8)
        row = self.row(); self.check(row, 'model_tools', self.tr('启用模型工具调用（Agent）', 'Enable model tools (Agent)'), p.get('modelTools', False))
        self.button(row, 'settings_save', self.tr('保存并加载', 'Save & load'), self.save_settings, True).pack(side='right')
        self.button(row, 'settings_validate', self.tr('重新验证', 'Validate again'), lambda: self.action('validate', callback=lambda r: self.poll_validation())).pack(side='right', padx=8)
        self.label(self.content, self.tr('密钥只从启动环境读取。项目只登记路径，个人配置与草稿升级保留。', 'Keys come from the startup environment. Projects store paths; personal data survives upgrades.'))
        self.editor(self.content, 'validation', pretty(self.preferences.get('validation')), 7, True)
        self.editor(self.content, 'storage', '', 3, True)
        self.action('storage-usage', callback=lambda r: set_text(self.widgets['storage'], pretty(r)) if self.page == 'settings' else None)
        self.poll_validation()

    def save_settings(self):
        p = copy.deepcopy(self.preferences['preferences'])
        for key in ['modelUrl', 'model', 'keyEnv', 'shell', 'workspace', 'setup', 'threshold', 'pollSeconds']: p[key] = self.value('setting_'+key)
        p.update(threshold=float(p['threshold']), pollSeconds=int(p['pollSeconds']), modelTools=bool(self.value('model_tools')), configured=True)
        self.action('save-preferences', p, lambda r: self.poll_validation())

    def poll_validation(self):
        def got(r):
            self.preferences = r
            if self.page == 'settings': set_text(self.widgets['validation'], pretty(r.get('validation')))
        self.action('preferences', callback=got)

    def tick(self):
        if self.closed: return
        if self.lifecycle and self.lifecycle.sock:
            try:
                if self.lifecycle.sock.recv(16) == b'focus': self.root.deiconify(); self.root.lift(); self.root.focus_force()
            except socket_error: pass
        self.capture()
        if self.dirty and not self.saving and not self.save_error: self.save_workspace()
        now = int(time.time())
        interval = int(self.preferences['preferences'].get('pollSeconds') or 5) if self.page == 'monitor' else 15
        if now % interval == 0 and self.root.state() != 'iconic': self.poll_monitor()
        if self.page == 'settings' and now % 3 == 0: self.poll_validation()
        self.root.after(1000, self.tick)

    def close(self):
        if self.closed: return
        if (self.agent or self.eda_run or self.code_run) and not messagebox.askyesno('IC Workbench', self.tr('仍有任务运行。退出会停止本客户端启动的后台，继续？', 'Tasks are active. Closing stops the backend started by this client. Continue?'), parent=self.root): return
        self.capture()
        if self.save_error:
            if not messagebox.askyesno('IC Workbench', self.tr('草稿尚未保存，可先通过“恢复草稿”导出。仍然退出？', 'Drafts are not saved. Use Recover drafts to export first. Exit anyway?'), parent=self.root): return
            self.finish_close(); return
        if self.dirty or self.saving: self.save_workspace(self.finish_close)
        else: self.finish_close()

    def finish_close(self):
        self.closed = True; self.workers.closed = True; self.root.destroy()

import socket
socket_error = socket.error

def main():
    os.umask(0o077)
    if not os.environ.get('DISPLAY'):
        sys.stderr.write('Native desktop requires an X11 desktop, VNC or SSH X forwarding (DISPLAY).\n')
        return 1
    package = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
    lifecycle = None; root = None
    try:
        root = tk.Tk(); root.withdraw()
        lifecycle = Lifecycle(package, [v for v in sys.argv[1:] if v != '--open'])
        if not lifecycle.claim(): return 0
        api, metadata = lifecycle.connect()
        app = Workbench(root, api, metadata, lifecycle); root.deiconify()
        signal.signal(signal.SIGTERM, lambda *args: root.after(0, app.close))
        signal.signal(signal.SIGINT, lambda *args: root.after(0, app.close))
        root.mainloop(); return 0
    except Exception as error:
        sys.stderr.write(str(error)+'\n')
        if root:
            try: messagebox.showerror('IC Workbench', str(error), parent=root)
            except tk.TclError: pass
        return 1
    finally:
        if lifecycle: lifecycle.close()

if __name__ == '__main__': sys.exit(main())
