# -*- coding: utf-8 -*-
"""Native engineering dialogs, compatible with Python 2.6 / Tk 8.5."""
from __future__ import unicode_literals
import json, difflib
from runtime import tk, filedialog, messagebox, BG, PAPER, GREEN, INK, MUTED, Table, read_text, text_value, set_text, export_text, pretty, as_text

SAMPLE = 'metric,value,unit,corner,voltage,temperature,condition,min,max\nbandwidth,1,MHz,tt,1.8,25,ac,0.9,1.2\npower,2,mW,tt,1.8,25,dc,,2.5\n'

def diff_text(before, after):
    if before == after: return '没有变化 / No changes'
    return ''.join(difflib.unified_diff(before.splitlines(True), after.splitlines(True), 'original', 'proposal'))

class Engineering(object):
    def engineering_data(self):
        if not isinstance(self.data.get('engineering'), dict): self.data['engineering'] = {}
        return self.data['engineering']

    def engineering_window(self, title):
        w = tk.Toplevel(self.root); w.title(title); w.geometry('1040x780'); w.configure(bg=BG)
        w.transient(self.root)
        return w

    def engineering_text(self, parent, value='', height=5, readonly=False):
        f = tk.Frame(parent, bg=BG); f.pack(fill='both', expand=True, padx=12, pady=5)
        t = tk.Text(f, height=height, wrap='word', bg=PAPER, fg=INK, font=('DejaVu Sans Mono', 10))
        scroll = tk.Scrollbar(f, command=t.yview); scroll.pack(side='right', fill='y')
        t.configure(yscrollcommand=scroll.set); t.pack(fill='both', expand=True); t.insert('1.0', value)
        if readonly: t.configure(state='disabled')
        return t

    def engineering_button(self, parent, label, fn):
        b = tk.Button(parent, text=label, command=lambda: self.safe(fn), bg=PAPER, fg=GREEN, padx=10, pady=7)
        b.pack(side='left', padx=5, pady=4); return b

    def task_templates(self):
        def loaded(rows):
            w = self.engineering_window(self.tr('参数化任务模板', 'Parameterized tasks')); self.template_window = w
            self.label(w, self.tr('选择模板 → 填参数 → 预览 → 带回 AI 对话；保持当前权限。', 'Choose → fill → preview → use in AI chat; permissions stay unchanged.'))
            row = tk.Frame(w, bg=BG); row.pack(fill='x'); var = tk.StringVar(value=rows[0]['name'])
            body = tk.Frame(w, bg=BG); body.pack(fill='x', padx=12)
            preview = self.engineering_text(w, '', 8, True); fields = {}; selected = {}; result = {}
            def render(name):
                for child in body.winfo_children(): child.destroy()
                selected.clear(); selected.update(next(t for t in rows if t['name'] == name)); fields.clear(); result.clear()
                saved = self.engineering_data().get('templateValues') or {}
                specs = [(p['name'], p['label'], saved.get(p['name'], p['default'])) for p in selected['parameters']]
                specs += [('maxRounds', self.tr('最大推理轮数 1–12', 'Maximum rounds 1–12'), '4'), ('maxSeconds', self.tr('最长秒数 10–600', 'Maximum seconds 10–600'), '120')]
                for key, label, value in specs:
                    self.label(body, label); t = tk.Entry(body, bg=PAPER); t.insert(0, value); t.pack(fill='x'); fields[key] = t
            tk.OptionMenu(row, var, *[t['name'] for t in rows], **{'command':render}).pack(side='left'); render(rows[0]['name'])
            def payload():
                return {'id': selected['id'], 'values':dict((p['name'],fields[p['name']].get()) for p in selected['parameters']), 'maxRounds':int(fields['maxRounds'].get()), 'maxSeconds':int(fields['maxSeconds'].get())}
            def show():
                data = payload(); self.engineering_data().update(templateID=data['id'], templateValues=data['values']); self.changed()
                def got(r):
                    if not w.winfo_exists(): return
                    result.clear(); result.update(r); result['input'] = data; set_text(preview, '\n'.join(r['steps'])+'\n\n'+r['text'])
                self.action('preview-task-template', data, got)
            def use():
                if not result or result['input'] != payload(): raise ValueError(self.tr('参数变化后需重新预览', 'Preview the current parameters first'))
                if self.agent: raise ValueError('Wait for current task')
                self.data.setdefault('chat', {})['text'] = result['text']; self.engineering_data()['nextBudget'] = dict((k,result[k]) for k in ('maxRounds','maxSeconds'))
                self.engineering_data()['nextBudget'].update(templateID=result['input']['id'],templateValues=result['input']['values'])
                self.changed(); w.destroy(); self.goto('chat', False)
            def custom():
                t = selected
                def save(v):
                    self.action('save-task-template', {'name':v['name'],'prompt':v['prompt'],'parameters':json.loads(v['parameters'])}, lambda r:self.info(self.tr('个人模板已保存，重新打开列表可使用', 'Template saved; reopen the list to use it')))
                self.form(self.tr('另存个人模板', 'Save personal template'), [('name','名称 / Name',''),('prompt','提示词 {{parameter}} / Prompt','multiline'),('parameters','参数 JSON: name, label, default','multiline')], {'name':t['name']+' copy','prompt':t['prompt'],'parameters':pretty(t['parameters'])},save)
            bottom = tk.Frame(w,bg=BG);bottom.pack(fill='x');self.engineering_button(bottom,self.tr('预览任务','Preview task'),show);self.engineering_button(bottom,self.tr('填入 AI 对话','Use in AI chat'),use);self.engineering_button(bottom,self.tr('另存个人模板','Save personal template'),custom)
            self.template_fields=fields; self.template_preview=show; self.template_use=use
        self.action('task-templates', callback=loaded)

    def metric_compare(self):
        w = self.engineering_window(self.tr('指标比较 · 单位与工况','Metric comparison · units & conditions'));self.metric_window=w
        self.label(w,self.tr('CSV：metric,value,unit,corner,voltage,temperature,condition,min,max；电源 V，温度 °C。阈值取基线。','CSV: metric,value,unit,corner,voltage,temperature,condition,min,max. Voltage V; temperature °C; baseline limits.'))
        d=self.engineering_data(); pane=tk.Frame(w,bg=BG);pane.pack(fill='both',expand=True);fields={}
        for kind,zh,en in [('baseline','基线 CSV','Baseline CSV'),('candidate','候选 CSV','Candidate CSV')]:
            f=tk.Frame(pane,bg=BG);f.pack(side='left',fill='both',expand=True);self.label(f,self.tr(zh,en));fields[kind]=self.engineering_text(f,d.get(kind,''),7)
            def load(k=kind):
                p=filedialog.askopenfilename(parent=w,filetypes=[('CSV','*.csv')])
                if p: set_text(fields[k],read_text(p,256000));d[k+'Name']=p
            row=tk.Frame(f,bg=BG);row.pack(fill='x');self.engineering_button(row,self.tr('导入 CSV','Import CSV'),load)
        table=Table(w,['Metric / PVT','Baseline','Candidate','Delta','Status / Source'],height=8);table.pack(fill='both',expand=True,padx=12)
        detail=self.engineering_text(w,'',4,True)
        def show(r):
            d['metricResult']=r;self.changed()
            if not w.winfo_exists():return
            table.set(r['rows'],lambda x:[x['metric']+' / '+x['condition'],as_text(x.get('baseline','—'))+' '+x['unit'],as_text(x.get('candidate','—'))+' '+x['unit'],as_text(x.get('delta','—')),x['status']+' / '+x['reason']])
            set_text(detail,r['note']+'\n\n'+'\n'.join(x['metric']+' · '+x['reason']+' · '+x['source'] for x in r['rows']))
        def compare():
            for k in fields:d[k]=text_value(fields[k])
            self.changed();self.action('compare-metrics',dict((k,d.get(k,'')) for k in ('baseline','candidate','baselineName','candidateName')),show)
        def sample():set_text(fields['baseline'],SAMPLE);set_text(fields['candidate'],SAMPLE.replace('1,MHz','1100,kHz').replace('2,mW','1800,uW'))
        row=tk.Frame(w,bg=BG);row.pack(fill='x');self.engineering_button(row,self.tr('合成样例','Synthetic sample'),sample);self.engineering_button(row,self.tr('核对工况并比较','Compare matching conditions'),compare);self.engineering_button(row,self.tr('导出比较结果','Export result'),lambda:export_text(w,pretty(d.get('metricResult',{})),'metric-comparison.json'))
        if d.get('metricResult'):show(d['metricResult'])
        self.metric_fields=fields;self.metric_sample=sample;self.metric_run=compare;self.metric_table=table

    def change_sets(self):
        def loaded(rows):
            w=self.engineering_window(self.tr('AI 多文件修改集','AI multi-file change sets'));self.changes_window=w;d=self.engineering_data()
            self.label(w,self.tr('最多 8 个相对路径 / 合计 120 KB。先生成候选，再逐文件审阅采用；不会自动提交 Git。','Up to 8 relative paths / 120 KB total. Propose, review and apply per file; no automatic Git commit.'))
            self.label(w,self.tr('文件所在目录（绝对路径）','Source directory (absolute path)'));root=tk.Entry(w,bg=PAPER);root.insert(0,d.get('changeRoot',''));root.pack(fill='x',padx=12)
            row=tk.Frame(w,bg=BG);row.pack(fill='x')
            def choose():
                p=filedialog.askdirectory(parent=w)
                if p:root.delete(0,'end');root.insert(0,p)
            self.engineering_button(row,self.tr('浏览目录','Browse directory'),choose)
            self.label(w,self.tr('相对文件路径，每行一个','Relative file paths, one per line'));paths=self.engineering_text(w,d.get('changePaths','rtl/top.sv\nscripts/run.tcl'),3)
            self.label(w,self.tr('跨文件修改需求','Coordinated change requirements'));req=self.engineering_text(w,d.get('changeRequirements',''),3)
            progress=tk.StringVar(value='');tk.Label(w,textvariable=progress,bg=BG,fg=GREEN).pack(fill='x')
            def generate():
                d.update(changeRoot=root.get(),changePaths=text_value(paths),changeRequirements=text_value(req));self.changed();progress.set(self.tr('正在读取文件并生成候选…','Reading files and generating proposal…'));generate_button.configure(state='disabled')
                def got(c):
                    if w.winfo_exists(): w.destroy()
                    self.change_review(c)
                def failed(e):
                    if w.winfo_exists():generate_button.configure(state='normal');progress.set(as_text(e))
                    self.error(e)
                self.action('propose-change-set',{'root':d['changeRoot'],'paths':[p.strip() for p in d['changePaths'].splitlines() if p.strip()],'requirements':d['changeRequirements']},got,failure=failed)
            row=tk.Frame(w,bg=BG);row.pack(fill='x');generate_button=self.engineering_button(row,self.tr('生成候选，保留原文件','Propose; preserve originals'),generate)
            table=Table(w,['Name','Created','Mode'],height=5);table.pack(fill='both',expand=True,padx=12);table.set(rows,lambda c:[c['name'],c['created'],'DEMO' if c['demo'] else 'Linux'])
            def open_selected():
                c=table.selected()
                if c:self.action('change-set',{'id':c['id']},self.change_review)
            table.view.bind('<Double-1>',lambda e:open_selected());row=tk.Frame(w,bg=BG);row.pack(fill='x');self.engineering_button(row,self.tr('打开选中修改集','Open selected set'),open_selected)
            self.changes_fields={'root':root,'paths':paths,'requirements':req};self.changes_generate=generate
        self.action('change-sets',callback=loaded)

    def change_review(self,c,selected_path=None):
        w=self.engineering_window(self.tr('逐文件审阅','Review each file'));self.change_review_window=w;self.review_set=c
        self.label(w,c['summary']).configure(wraplength=980,justify='left');table=Table(w,['File','State'],height=4);table.pack(fill='x',padx=12);table.set(c['files'],lambda f:[f['path'],f['state']])
        diff=self.engineering_text(w,'',5,True)
        pane=tk.Frame(w,bg=BG);pane.pack(fill='both',expand=True);texts=[]
        for label in [self.tr('原文件','Original'),self.tr('AI 候选（未验证）','AI proposal (unverified)')]:
            f=tk.Frame(pane,bg=BG);f.pack(side='left',fill='both',expand=True);self.label(f,label);texts.append(self.engineering_text(f,'',10,True))
        def select(event=None):
            f=table.selected()
            if f:set_text(diff,diff_text(f['before'],f['after']));set_text(texts[0],f['before']);set_text(texts[1],f['after'])
        table.view.bind('<<TreeviewSelect>>',select)
        def modify(action):
            f=table.selected()
            if not f:raise ValueError(self.tr('请先选择文件','Select a file first'))
            def got(updated):
                if w.winfo_exists():w.destroy()
                self.change_review(updated,f['path'])
            self.action(action,{'id':c['id'],'path':f['path']},got)
        row=tk.Frame(w,bg=BG);row.pack(fill='x');self.engineering_button(row,self.tr('采用此文件','Apply this file'),lambda:modify('apply-change-file'));self.engineering_button(row,self.tr('撤销此文件','Undo this file'),lambda:modify('undo-change-file'));self.engineering_button(row,self.tr('导出审阅记录','Export review'),lambda:export_text(w,pretty(c),'change-set-'+c['id']+'.json'))
        def remove():
            if messagebox.askyesno('IC Workbench',self.tr('仅删除本机候选和撤销记录，不能恢复。项目文件保持现状。请先导出需要的记录，继续？','Permanently remove local proposal and undo history? Project files stay unchanged. Export first if needed.'),parent=w):
                self.action('archive-change-set',{'id':c['id']},lambda r:w.destroy())
        self.engineering_button(row,self.tr('移除此审阅记录','Remove review record'),remove)
        self.review_table=table;self.review_apply=lambda:modify('apply-change-file');self.review_undo=lambda:modify('undo-change-file')
        if c['files']:
            index=next((i for i,item in enumerate(c['files']) if item['path']==selected_path),0)
            table.select(index);select()

    def chat_details(self):
        w=self.engineering_window(self.tr('原始对话与工具回执','Original conversation and tool receipts'))
        self.engineering_text(w,getattr(self,'chat_raw','')+'\n\n'+pretty(self.agent_snapshot or {}),28,True)


def readable_chat(text):
    out=[]
    for line in text.splitlines():
        value=line.strip()
        if value.startswith('{') or value.startswith('['):
            try:
                json.loads(value)
                out.append('〔结构化工具数据：请点“查看原始回执” / Structured data: see original receipts〕')
                continue
            except ValueError: pass
        out.append(line)
    return '\n'.join(out)
