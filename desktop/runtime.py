# -*- coding: utf-8 -*-
"""Native desktop runtime. Python 2.6+; standard library and system Tk only."""
from __future__ import unicode_literals
import os, sys, json, stat, time, tempfile, subprocess, threading, socket, fcntl, re
try:
    import Tkinter as tk
    import tkFileDialog as filedialog
    import tkMessageBox as messagebox
    import Queue as queue
    import urllib2 as urlrequest
except ImportError:
    import tkinter as tk
    from tkinter import filedialog, messagebox
    import queue
    import urllib.request as urlrequest

BG = '#f7f8f4'
SIDE = '#eaf1e9'
PAPER = '#ffffff'
GREEN = '#366c58'
INK = '#253a32'
MUTED = '#76887d'

def as_text(value):
    try:
        return unicode(value)
    except NameError:
        return str(value)
    except UnicodeError:
        return repr(value).decode('utf-8', 'replace')

def pretty(value):
    return json.dumps(value, ensure_ascii=False, indent=2)

def workspace_defaults(value):
    value = value or {}
    for key in ('eda', 'codeDrafts', 'calculator', 'chat'):
        if value.get(key) is None:
            value[key] = {}
    for key in ('codeArchive', 'analyses'):
        if value.get(key) is None:
            value[key] = []
    return value

def private_dir(path):
    if not os.path.exists(path):
        os.makedirs(path, 0o700)
    s = os.lstat(path)
    if not stat.S_ISDIR(s.st_mode) or s.st_uid != os.getuid() or stat.S_IMODE(s.st_mode) != 0o700:
        raise ValueError('Private directory must belong to current UID with mode 0700: ' + path)

def private_json(path):
    fd = os.open(path, os.O_RDONLY | getattr(os, 'O_NOFOLLOW', 0))
    try:
        s = os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid != os.getuid() or stat.S_IMODE(s.st_mode) != 0o600 or s.st_nlink != 1:
            raise ValueError('Unsafe private record: ' + path)
        raw = os.read(fd, 1048577)
        if len(raw) > 1048576:
            raise ValueError('Private record too large')
        return json.loads(raw.decode('utf-8'))
    finally:
        os.close(fd)

class API(object):
    def __init__(self, instance):
        self.instance = instance
        self.base = instance['url']
        if not re.match(r'^http://(127\.0\.0\.1|\[::1\]):[0-9]+$', self.base):
            raise ValueError('Desktop API must be local')
        if instance.get('mode') != 'production' or instance['uid'] != os.getuid() or not re.match(r'^[a-f0-9]{48}$', instance['token']):
            raise ValueError('Invalid private instance')

    def request(self, route, data=None):
        raw = None if data is None else json.dumps(data).encode('utf-8')
        req = urlrequest.Request(self.base + '/api/' + route, raw, {
            'Authorization': 'Bearer ' + self.instance['token'], 'Content-Type': 'application/json'})
        opener = urlrequest.build_opener(urlrequest.ProxyHandler({}))
        try:
            response = opener.open(req, timeout=150)
            try:
                body = response.read(16 * 1024 * 1024 + 1)
            finally:
                response.close()
        except urlrequest.HTTPError as error:
            detail = error.read(8192).decode('utf-8', 'replace')
            try:
                detail = json.loads(detail).get('error', detail)
            except ValueError:
                pass
            raise ValueError(detail)
        if len(body) > 16 * 1024 * 1024:
            raise ValueError('Response exceeds desktop limit')
        return json.loads(body.decode('utf-8'))

    def action(self, action, data=None):
        return self.request('v2/action', {'action': action, 'input': data or {}})

class Workers(object):
    """Bounded workers; all Tk access and callbacks stay on the GUI thread."""
    def __init__(self, root, on_error):
        self.root, self.on_error = root, on_error
        self.pending, self.results = queue.Queue(48), queue.Queue()
        self.closed = False
        for i in range(3):
            t = threading.Thread(target=self.work)
            t.daemon = True
            t.start()
        root.after(80, self.drain)

    def work(self):
        while True:
            item = self.pending.get()
            if item is None:
                return
            fn, callback, failure = item
            try:
                result = fn()
                self.results.put((callback, result, None, failure))
            except Exception as error:
                self.results.put((callback, None, error, failure))

    def submit(self, fn, callback=None, failure=None):
        if self.closed:
            return
        try:
            self.pending.put_nowait((fn, callback, failure))
        except queue.Full:
            # Rejected requests must complete their own failure lifecycle too:
            # saves and resource polling otherwise remain permanently in-flight.
            (failure or self.on_error)(ValueError('Request queue full; wait for active tasks'))

    def drain(self):
        if self.closed:
            return
        for i in range(32):
            try:
                callback, value, error, failure = self.results.get_nowait()
            except queue.Empty:
                break
            try:
                if error:
                    (failure or self.on_error)(error)
                elif callback:
                    callback(value)
            except Exception as error:
                self.on_error(error)
        self.root.after(80, self.drain)

class Table(tk.Frame):
    """Use Tk 8.5's native ttk treeview without requiring Python 2.7 ttk."""
    def __init__(self, parent, columns, callback=None, height=10):
        tk.Frame.__init__(self, parent, bg=BG)
        self.view = tk.Widget(self, 'ttk::treeview', {'columns': tuple('c%d' % n for n in range(len(columns))), 'show': 'headings', 'height': height, 'selectmode': 'browse'})
        self.view.pack(side='left', fill='both', expand=True)
        scroll = tk.Scrollbar(self, command=lambda *v: self.view.tk.call(self.view._w, 'yview', *v))
        scroll.pack(side='right', fill='y')
        self.view.configure(yscrollcommand=scroll.set)
        self.rows = []
        for i, col in enumerate(columns):
            self.view.tk.call(self.view._w, 'heading', 'c%d' % i, '-text', col)
            self.view.tk.call(self.view._w, 'column', 'c%d' % i, '-width', 145, '-minwidth', 55)
        if callback:
            self.view.bind('<<TreeviewSelect>>', lambda event: callback(self.selected()))

    def set(self, rows, values):
        selected = self.index()
        children = self.view.tk.call(self.view._w, 'children', '')
        if children:
            self.view.tk.call(self.view._w, 'delete', children)
        self.rows = rows
        for i, row in enumerate(rows):
            self.view.tk.call(self.view._w, 'insert', '', 'end', '-id', str(i), '-values', tuple(values(row)))
        if selected is not None and selected < len(rows):
            self.view.tk.call(self.view._w, 'selection', 'set', str(selected))

    def index(self):
        result = self.view.tk.call(self.view._w, 'selection')
        result = self.view.tk.splitlist(result)
        return int(result[0]) if result else None

    def selected(self):
        i = self.index()
        return self.rows[i] if i is not None and i < len(self.rows) else None

    def select(self, index):
        self.view.tk.call(self.view._w, 'selection', 'set', str(index))

def text_value(widget):
    return widget.get('1.0', 'end-1c')

def set_text(widget, text):
    state = str(widget.cget('state'))
    widget.configure(state='normal')
    widget.delete('1.0', 'end')
    widget.insert('1.0', text or '')
    widget.configure(state=state)

def export_text(parent, text, name):
    path = filedialog.asksaveasfilename(parent=parent, initialfile=name, confirmoverwrite=True)
    if not path:
        return
    folder = os.path.dirname(os.path.abspath(path))
    fd, temporary = tempfile.mkstemp(prefix='.icw-export-', dir=folder)
    try:
        f = os.fdopen(fd, 'wb')
        try:
            f.write(text.encode('utf-8')); f.flush(); os.fsync(f.fileno())
        finally:
            f.close()
        os.rename(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)

def read_text(path, limit):
    with open(path, 'rb') as inp:
        value = inp.read(limit + 1)
    if len(value) > limit:
        raise ValueError('File exceeds import limit')
    if b'\x00' in value:
        raise ValueError('Binary file: export UTF-8 text / CSV / VCD first')
    return value.decode('utf-8-sig')

def argument(args, name, default):
    for i in range(len(args) - 1, -1, -1):
        if args[i].startswith(name + '='):
            return args[i].split('=', 1)[1]
        if args[i] == name and i + 1 < len(args):
            return args[i + 1]
    return default

class Lifecycle(object):
    def __init__(self, package, args):
        self.package, self.args = package, args
        self.child, self.log, self.lock, self.sock = None, None, None, None
        config = argument(args, '--config', os.path.join(os.environ.get('ICW_CONFIG_DIR', os.path.expanduser('~/.config/ic-workbench/production')), 'config.json'))
        configured = private_json(config) if os.path.exists(config) else {}
        self.data = os.path.abspath(argument(args, '--data', configured.get('dataDir') or os.path.expanduser('~/.local/share/ic-workbench/production')))
        private_dir(self.data)

    def claim(self):
        path = os.path.join(self.data, '.desktop.lock')
        fd = os.open(path, os.O_CREAT | os.O_RDWR | getattr(os, 'O_NOFOLLOW', 0), 0o600)
        s = os.fstat(fd)
        if s.st_uid != os.getuid() or not stat.S_ISREG(s.st_mode) or s.st_nlink != 1 or stat.S_IMODE(s.st_mode) != 0o600:
            os.close(fd)
            raise ValueError('Unsafe desktop lock')
        self.lock = os.fdopen(fd, 'a+')
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except IOError:
            # A local datagram never carries an executable instruction or credentials.
            sender = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
            try:
                sender.sendto(b'focus', os.path.join(self.data, '.desktop.sock'))
            finally:
                sender.close()
            return False
        sockpath = os.path.join(self.data, '.desktop.sock')
        if os.path.lexists(sockpath):
            s = os.lstat(sockpath)
            if not stat.S_ISSOCK(s.st_mode) or s.st_uid != os.getuid():
                raise ValueError('Unsafe desktop socket')
            os.unlink(sockpath)
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
        self.sock.bind(sockpath); self.sock.setblocking(False)
        os.chmod(sockpath, 0o600)
        return True

    def connect(self):
        receipt = os.path.join(self.data, '.instance.json')
        try:
            api = API(private_json(receipt)); meta = api.request('state')
            if meta['version'] != '0.7.0-rc1':
                raise ValueError('Stop the previous platform version before opening this desktop client')
            return api, meta
        except (IOError, OSError):
            pass
        self.log = tempfile.TemporaryFile()
        self.child = subprocess.Popen([os.path.join(self.package, 'service.sh'), '--desktop-api'] + self.args,
                                      stdin=open(os.devnull), stdout=self.log, stderr=subprocess.STDOUT)
        deadline = time.time() + 20
        while time.time() < deadline:
            if self.child.poll() is not None:
                self.log.seek(0)
                raise ValueError(self.log.read(65536).decode('utf-8', 'replace'))
            try:
                api = API(private_json(receipt)); meta = api.request('state')
                if api.instance['pid'] != self.child.pid:
                    # A concurrent service won the lock; never claim ownership of it.
                    self.child.wait(); self.child = None
                return api, meta
            except (IOError, OSError, ValueError):
                time.sleep(.1)
        raise ValueError('Desktop backend startup timed out')

    def close(self):
        if self.child is not None and self.child.poll() is None:
            self.child.terminate()
            deadline = time.time() + 8
            while self.child.poll() is None and time.time() < deadline:
                time.sleep(.05)
            if self.child.poll() is None:
                self.child.kill(); self.child.wait()
        if self.log:
            self.log.close()
        if self.sock:
            self.sock.close()
            os.unlink(os.path.join(self.data, '.desktop.sock'))
        if self.lock:
            self.lock.close()
