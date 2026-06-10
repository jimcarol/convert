const form = document.getElementById('note-form');
const notesDiv = document.getElementById('notes');
const noteIdInput = document.getElementById('note-id');
const titleInput = document.getElementById('title');
const contentInput = document.getElementById('content');
const cancelBtn = document.getElementById('cancel-edit');
const tagOptions = document.getElementById('tag-options');
const showMoreBtn = document.getElementById('show-more');
const showLessBtn = document.getElementById('show-less');
const loginBtn = document.getElementById('login-btn');
const loggedInArea = document.getElementById('logged-in-area');
const loginDialog = document.getElementById('login-dialog');
const loginPasswordInput = document.getElementById('login-password');
const loginError = document.getElementById('login-error');
const loginConfirmBtn = document.getElementById('login-confirm');
const loginCancelBtn = document.getElementById('login-cancel');
const logoutBtn = document.getElementById('logout-btn');
const modalOverlay = document.getElementById('modal-overlay');
const modalContent = document.getElementById('modal-content');
const modalBody = document.getElementById('modal-body');
const modalClose = document.getElementById('modal-close');
const VISIBLE_COUNT = 5;
const AVAILABLE_TAGS = ['work', 'personal', 'idea', 'todo'];
const DEFAULT_TAG = 'default';

let isAuthenticated = localStorage.getItem('authenticated') === 'true';
let currentNotes = [];

marked.setOptions({
  gfm: true,
  breaks: true
});

function updateAuthUI() {
  loginBtn.style.display = isAuthenticated ? 'none' : 'inline-flex';
  loggedInArea.style.display = isAuthenticated ? 'inline-flex' : 'none';
}

function toggleTextareaHeight() {
  contentInput.classList.toggle('has-content', contentInput.value.trim().length > 0);
}

function showLoginDialog() {
  loginDialog.style.display = 'block';
  loginPasswordInput.focus();
}

function hideLoginDialog() {
  loginDialog.style.display = 'none';
  loginError.style.display = 'none';
  loginPasswordInput.value = '';
}

async function doLogin() {
  const pw = loginPasswordInput.value;
  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pw })
    });
    if (res.ok) {
      isAuthenticated = true;
      localStorage.setItem('authenticated', 'true');
      hideLoginDialog();
      updateAuthUI();
      fetchNotes();
      return;
    }
  } catch {
  }
  loginError.style.display = 'inline';
}

async function doLogout() {
  await fetch('/api/logout', { method: 'POST' });
  isAuthenticated = false;
  localStorage.removeItem('authenticated');
  updateAuthUI();
  fetchNotes();
}

function handleAuthError() {
  isAuthenticated = false;
  localStorage.removeItem('authenticated');
  updateAuthUI();
  fetchNotes();
}

function getLocalNotes() {
  const notes = JSON.parse(localStorage.getItem('local_notes') || '[]');
  return Array.isArray(notes) ? notes.map(normalizeNote) : [];
}

function saveLocalNotes(notes) {
  localStorage.setItem('local_notes', JSON.stringify(notes));
}

function normalizeTags(tags) {
  if (!Array.isArray(tags)) {
    return [];
  }

  const seen = new Set();
  return tags
    .map((tag) => String(tag || '').trim().toLowerCase())
    .filter((tag) => AVAILABLE_TAGS.includes(tag))
    .filter((tag) => {
      if (seen.has(tag)) {
        return false;
      }
      seen.add(tag);
      return true;
    });
}

function getEffectiveTags(note) {
  const tags = normalizeTags(note?.tags);
  return tags.length > 0 ? tags : [DEFAULT_TAG];
}

function normalizeNote(note) {
  return {
    ...note,
    tags: normalizeTags(note?.tags)
  };
}

function getSelectedFormTags() {
  return Array.from(tagOptions.querySelectorAll('input[type="checkbox"]:checked'))
    .map((input) => input.value);
}

function setSelectedFormTags(tags) {
  const selected = new Set(normalizeTags(tags));
  tagOptions.querySelectorAll('input[type="checkbox"]').forEach((input) => {
    input.checked = selected.has(input.value);
  });
}

function createTagPickerOption(tag, name, checked = false) {
  const label = document.createElement('label');
  label.className = 'tag-option';

  const input = document.createElement('input');
  input.type = 'checkbox';
  input.name = name;
  input.value = tag;
  input.checked = checked;

  const text = document.createElement('span');
  text.className = 'tag-option-text';
  text.textContent = tag;

  label.appendChild(input);
  label.appendChild(text);
  return label;
}

function renderFormTagPicker(tags = []) {
  tagOptions.innerHTML = '';
  const selected = new Set(normalizeTags(tags));
  AVAILABLE_TAGS.forEach((tag) => {
    tagOptions.appendChild(createTagPickerOption(tag, 'note-tags', selected.has(tag)));
  });
}

function normalizeCodeLanguage(language) {
  const value = (language || '').trim().toLowerCase();
  if (!value) return '';

  const aliases = {
    js: 'javascript',
    ts: 'typescript',
    sh: 'bash',
    shell: 'bash',
    yml: 'yaml',
    rb: 'ruby'
  };

  return aliases[value] || value;
}

function escapeHtml(value) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function buildMarkdownHtml(source) {
  const renderer = new marked.Renderer();
  renderer.link = function ({ href, title, tokens }) {
    const text = this.parser.parseInline(tokens);
    const safeHref = typeof href === 'string' ? href : '';
    const titleAttr = title ? ` title="${escapeHtml(title)}"` : '';
    return `<a href="${escapeHtml(safeHref)}"${titleAttr}>${text}</a>`;
  };
  renderer.code = function ({ text, lang }) {
    const language = normalizeCodeLanguage(lang);
    const className = language ? ` class="language-${language}"` : '';
    return `<pre><code${className}>${escapeHtml(text)}</code></pre>`;
  };

  return marked.parse(source || '', { renderer });
}

function sanitizeRenderedHtml(html) {
  const clean = DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    ADD_ATTR: ['class', 'target', 'rel', 'title']
  });

  const wrapper = document.createElement('div');
  wrapper.innerHTML = clean;

  wrapper.querySelectorAll('a').forEach((link) => {
    const href = link.getAttribute('href') || '';
    if (href && !/^(https?:|mailto:|tel:|\/|#)/i.test(href)) {
      link.removeAttribute('href');
      return;
    }
    if (/^https?:\/\//i.test(href)) {
      link.setAttribute('target', '_blank');
      link.setAttribute('rel', 'noopener noreferrer');
    }
  });

  return wrapper.innerHTML;
}

function renderMarkdownInto(container, source) {
  container.innerHTML = '';

  if (!source || !source.trim()) {
    const empty = document.createElement('p');
    empty.className = 'markdown-empty';
    empty.textContent = 'Empty note';
    container.appendChild(empty);
    return;
  }

  const rawHtml = buildMarkdownHtml(source);
  const cleanHtml = sanitizeRenderedHtml(rawHtml);
  container.innerHTML = cleanHtml;

  container.querySelectorAll('pre code').forEach((block) => {
    hljs.highlightElement(block);
  });
}

function getNoteById(id) {
  return currentNotes.find((note) => String(note.id) === String(id));
}

function createActionButton(label, className, onClick) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = className;
  button.textContent = label;
  button.addEventListener('click', (event) => {
    event.stopPropagation();
    onClick();
  });
  return button;
}

function createRenderedBody(source) {
  const body = document.createElement('div');
  body.className = 'markdown-body';
  renderMarkdownInto(body, source);
  return body;
}

async function saveTagsForNote(id, tags) {
  const normalizedTags = normalizeTags(tags);

  if (isAuthenticated) {
    const note = getNoteById(id);
    if (!note) {
      return;
    }

    const res = await fetch(`/notes/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title: note.title,
        content: note.content,
        tags: normalizedTags
      })
    });

    if (res.status === 401) {
      handleAuthError();
      return;
    }
  } else {
    const notes = getLocalNotes();
    const idx = notes.findIndex((entry) => String(entry.id) === String(id));
    if (idx === -1) {
      return;
    }

    notes[idx].tags = normalizedTags;
    notes[idx].updated_at = new Date().toISOString();
    saveLocalNotes(notes);
  }

  await fetchNotes();
}

function createTagChip(tag, options = {}) {
  const chip = document.createElement('span');
  chip.className = 'tag-chip';
  if (options.isDefault) {
    chip.classList.add('tag-chip-default');
  }

  const label = document.createElement('span');
  label.textContent = tag;
  chip.appendChild(label);

  if (options.removable) {
    const removeBtn = document.createElement('button');
    removeBtn.type = 'button';
    removeBtn.className = 'tag-chip-remove';
    removeBtn.textContent = '×';
    removeBtn.setAttribute('aria-label', `Remove tag ${tag}`);
    removeBtn.addEventListener('click', (event) => {
      event.stopPropagation();
      options.onRemove?.();
    });
    chip.appendChild(removeBtn);
  }

  return chip;
}

function createTagList(note) {
  const wrapper = document.createElement('div');
  wrapper.className = 'tag-list';

  const effectiveTags = getEffectiveTags(note);
  const actualTags = normalizeTags(note.tags);
  effectiveTags.forEach((tag) => {
    const removable = actualTags.includes(tag);
    wrapper.appendChild(createTagChip(tag, {
      isDefault: tag === DEFAULT_TAG,
      removable,
      onRemove: () => saveTagsForNote(note.id, actualTags.filter((value) => value !== tag))
    }));
  });

  return wrapper;
}

function createNoteTagEditor(note, onClose) {
  const editor = document.createElement('div');
  editor.className = 'note-tag-editor';

  const title = document.createElement('div');
  title.className = 'note-tag-editor-title';
  title.textContent = 'Update tags';

  const options = document.createElement('div');
  options.className = 'tag-picker-options';
  const selected = new Set(normalizeTags(note.tags));
  AVAILABLE_TAGS.forEach((tag) => {
    options.appendChild(createTagPickerOption(tag, `note-${note.id}-tags`, selected.has(tag)));
  });

  const actions = document.createElement('div');
  actions.className = 'note-tag-editor-actions';
  actions.appendChild(createActionButton('Apply', 'btn-note', async () => {
    const tags = Array.from(options.querySelectorAll('input[type="checkbox"]:checked')).map((input) => input.value);
    await saveTagsForNote(note.id, tags);
  }));
  actions.appendChild(createActionButton('Cancel', 'btn-note', onClose));

  editor.appendChild(title);
  editor.appendChild(options);
  editor.appendChild(actions);
  return editor;
}

async function fetchNotes() {
  let notes;
  if (isAuthenticated) {
    const res = await fetch('/notes');
    if (res.status === 401) {
      handleAuthError();
      return;
    }
    notes = await res.json();
  } else {
    notes = getLocalNotes();
  }

  currentNotes = Array.isArray(notes)
    ? notes.map(normalizeNote).sort((a, b) => (b.updated_at || '').localeCompare(a.updated_at || ''))
    : [];
  renderNotes();
}

function renderNotes() {
  notesDiv.innerHTML = '';

  currentNotes.forEach((note, index) => {
    const noteCard = document.createElement('div');
    noteCard.className = 'note';
    if (index >= VISIBLE_COUNT) {
      noteCard.style.display = 'none';
    }

    const header = document.createElement('div');
    header.className = 'note-header';

    const titleWrap = document.createElement('div');
    titleWrap.className = 'note-heading';

    const title = document.createElement('strong');
    title.className = 'note-title';
    title.textContent = `▶ ${note.title}`;

    const tags = createTagList(note);

    const actions = document.createElement('div');
    actions.className = 'note-actions';

    const content = document.createElement('div');
    content.className = 'note-content';

    const tagEditorHost = document.createElement('div');
    tagEditorHost.className = 'note-tag-editor-host';

    header.addEventListener('click', () => {
      content.classList.toggle('expanded');
    });

    actions.appendChild(createActionButton('Edit', 'btn-note', () => editNote(note.id)));
    actions.appendChild(createActionButton('Tags', 'btn-note', () => {
      if (tagEditorHost.childElementCount > 0) {
        tagEditorHost.innerHTML = '';
        return;
      }
      tagEditorHost.appendChild(createNoteTagEditor(note, () => {
        tagEditorHost.innerHTML = '';
      }));
    }));
    actions.appendChild(createActionButton('Delete', 'btn-note', () => deleteNote(note.id)));

    const body = createRenderedBody(note.content);

    const meta = document.createElement('div');
    meta.className = 'note-meta';

    const updated = document.createElement('small');
    updated.className = 'note-updated';
    updated.textContent = `Updated: ${new Date(note.updated_at).toLocaleString()}`;

    const expandBtn = createActionButton('Expand', 'btn-expand', () => expandNote(note.id));

    meta.appendChild(updated);
    meta.appendChild(expandBtn);

    titleWrap.appendChild(title);
    titleWrap.appendChild(tags);
    header.appendChild(titleWrap);
    header.appendChild(actions);
    content.appendChild(tagEditorHost);
    content.appendChild(body);
    content.appendChild(meta);
    noteCard.appendChild(header);
    noteCard.appendChild(content);
    notesDiv.appendChild(noteCard);
  });

  showMoreBtn.style.display = currentNotes.length > VISIBLE_COUNT ? 'block' : 'none';
  showLessBtn.style.display = 'none';
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();

  const id = noteIdInput.value;
  const note = {
    title: titleInput.value,
    content: contentInput.value,
    tags: normalizeTags(getSelectedFormTags())
  };

  if (isAuthenticated) {
    let res;
    if (id) {
      res = await fetch(`/notes/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(note)
      });
    } else {
      res = await fetch('/notes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(note)
      });
    }

    if (res.status === 401) {
      handleAuthError();
      return;
    }
  } else {
    const notes = getLocalNotes();
    const now = new Date().toISOString();

    if (id) {
      const idx = notes.findIndex((entry) => String(entry.id) === String(id));
      if (idx !== -1) {
        notes[idx].title = note.title;
        notes[idx].content = note.content;
        notes[idx].tags = note.tags;
        notes[idx].updated_at = now;
      }
    } else {
      note.id = Date.now();
      note.created_at = now;
      note.updated_at = now;
      notes.push(note);
    }

    saveLocalNotes(notes);
  }

  resetForm();
  fetchNotes();
});

async function deleteNote(id) {
  if (!window.confirm('Delete this note? This action cannot be undone.')) {
    return;
  }

  if (isAuthenticated) {
    const res = await fetch(`/notes/${id}`, { method: 'DELETE' });
    if (res.status === 401) {
      handleAuthError();
      return;
    }
  } else {
    const notes = getLocalNotes().filter((note) => String(note.id) !== String(id));
    saveLocalNotes(notes);
  }

  if (String(noteIdInput.value) === String(id)) {
    resetForm();
  }
  fetchNotes();
}

function editNote(id) {
  const note = getNoteById(id);
  if (!note) {
    return;
  }

  noteIdInput.value = note.id;
  titleInput.value = note.title;
  setSelectedFormTags(note.tags);
  contentInput.value = note.content;
  toggleTextareaHeight();
  cancelBtn.style.display = 'inline-flex';
  titleInput.focus();
}

function expandNote(id) {
  const note = getNoteById(id);
  if (!note) {
    return;
  }

  modalBody.innerHTML = '';

  const title = document.createElement('h2');
  title.className = 'modal-title';
  title.textContent = note.title;

  const body = createRenderedBody(note.content);
  modalBody.appendChild(title);
  modalBody.appendChild(body);

  modalOverlay.classList.add('active');
  modalContent.style.display = 'block';
}

function closeModal() {
  modalOverlay.classList.remove('active');
  modalContent.style.display = 'none';
}

function resetForm() {
  noteIdInput.value = '';
  titleInput.value = '';
  setSelectedFormTags([]);
  contentInput.value = '';
  contentInput.classList.remove('has-content');
  cancelBtn.style.display = 'none';
}

showMoreBtn.addEventListener('click', () => {
  notesDiv.querySelectorAll('.note').forEach((noteCard) => {
    noteCard.style.display = 'block';
  });
  showMoreBtn.style.display = 'none';
  showLessBtn.style.display = 'block';
});

showLessBtn.addEventListener('click', () => {
  notesDiv.querySelectorAll('.note').forEach((noteCard, index) => {
    noteCard.style.display = index >= VISIBLE_COUNT ? 'none' : 'block';
  });
  showLessBtn.style.display = 'none';
  showMoreBtn.style.display = currentNotes.length > VISIBLE_COUNT ? 'block' : 'none';
  window.scrollTo({ top: 0, behavior: 'smooth' });
});

cancelBtn.addEventListener('click', resetForm);
contentInput.addEventListener('input', toggleTextareaHeight);
loginBtn.addEventListener('click', showLoginDialog);
loginConfirmBtn.addEventListener('click', doLogin);
loginCancelBtn.addEventListener('click', hideLoginDialog);
logoutBtn.addEventListener('click', doLogout);
modalClose.addEventListener('click', closeModal);
modalOverlay.addEventListener('click', closeModal);
loginPasswordInput.addEventListener('keydown', (event) => {
  if (event.key === 'Enter') {
    doLogin();
  }
});

document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape' && modalOverlay.classList.contains('active')) {
    closeModal();
  }
});

renderFormTagPicker();
updateAuthUI();
toggleTextareaHeight();
fetchNotes();
