const form = document.getElementById('note-form');
const notesDiv = document.getElementById('notes');
const titleInput = document.getElementById('title');
const contentInput = document.getElementById('content');
const tagOptions = document.getElementById('tag-options');
const noteFormShell = document.getElementById('note-form-shell');
const noteFormTrigger = document.getElementById('note-form-trigger');
const noteFormHeading = document.getElementById('note-form-heading');
const noteFormSubheading = document.getElementById('note-form-subheading');
const noteFormTriggerIcon = document.getElementById('note-form-trigger-icon');
const notesLayout = document.getElementById('notes-layout');
const filterBar = document.getElementById('filter-bar');
const filterOptions = document.getElementById('filter-options');
const filterCount = document.getElementById('filter-count');
const clearFiltersBtn = document.getElementById('clear-filters');
const filterCollapsedCount = document.getElementById('filter-collapsed-count');
const toggleFilterBarBtn = document.getElementById('toggle-filter-bar');
const filterBarContent = document.getElementById('filter-bar-content');
const notesEmptyState = document.getElementById('notes-empty-state');
const showMoreBtn = document.getElementById('show-more');
const showLessBtn = document.getElementById('show-less');
const loginBtn = document.getElementById('login-btn');
const loggedInArea = document.getElementById('logged-in-area');
const loginDialog = document.getElementById('login-dialog');
const loginPasswordInput = document.getElementById('login-password');
const loginError = document.getElementById('login-error');
const resetCreateBtn = document.getElementById('reset-create');
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
const FILTER_TAGS_STORAGE_KEY = 'notes_filter_tags';
const FILTER_BAR_COLLAPSED_STORAGE_KEY = 'notes_filter_bar_collapsed';

let isAuthenticated = localStorage.getItem('authenticated') === 'true';
let currentNotes = [];
let selectedFilterTags = new Set();
let isFilterBarCollapsed = true;
let isNoteFormCollapsed = true;
let modalEditNoteId = null;
let modalMode = null;

marked.setOptions({
  gfm: true,
  breaks: true
});

function updateAuthUI() {
  loginBtn.style.display = isAuthenticated ? 'none' : 'inline-flex';
  loggedInArea.style.display = isAuthenticated ? 'inline-flex' : 'none';
}

function toggleTextareaHeight(textarea = contentInput) {
  textarea.classList.toggle('has-content', textarea.value.trim().length > 0);
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

function setExpiringPreference(key, value) {
  const expiresAt = Date.now() + 7 * 24 * 60 * 60 * 1000;
  localStorage.setItem(key, JSON.stringify({ value, expiresAt }));
}

function getExpiringPreference(key) {
  const raw = localStorage.getItem(key);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object') {
      localStorage.removeItem(key);
      return null;
    }
    if (typeof parsed.expiresAt !== 'number' || parsed.expiresAt <= Date.now()) {
      localStorage.removeItem(key);
      return null;
    }
    return parsed.value ?? null;
  } catch {
    localStorage.removeItem(key);
    return null;
  }
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

function getTagPickerRoot(root = tagOptions) {
  return root;
}

function getSelectedFormTags(root = tagOptions) {
  return Array.from(getTagPickerRoot(root).querySelectorAll('input[type="checkbox"]:checked'))
    .map((input) => input.value);
}

function setSelectedFormTags(tags, root = tagOptions) {
  const selected = new Set(normalizeTags(tags));
  getTagPickerRoot(root).querySelectorAll('input[type="checkbox"]').forEach((input) => {
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

function renderFormTagPicker(tags = [], root = tagOptions, name = 'note-tags') {
  const targetRoot = getTagPickerRoot(root);
  targetRoot.innerHTML = '';
  const selected = new Set(normalizeTags(tags));
  AVAILABLE_TAGS.forEach((tag) => {
    targetRoot.appendChild(createTagPickerOption(tag, name, selected.has(tag)));
  });
}

function getFilterableTags() {
  return [DEFAULT_TAG, ...AVAILABLE_TAGS];
}

function persistFilterPreferences() {
  setExpiringPreference(FILTER_TAGS_STORAGE_KEY, Array.from(selectedFilterTags));
  setExpiringPreference(FILTER_BAR_COLLAPSED_STORAGE_KEY, isFilterBarCollapsed);
}

function restoreFilterPreferences() {
  const savedTags = getExpiringPreference(FILTER_TAGS_STORAGE_KEY);
  if (Array.isArray(savedTags)) {
    selectedFilterTags = new Set(normalizeTags(savedTags));
  }

  const savedCollapsed = getExpiringPreference(FILTER_BAR_COLLAPSED_STORAGE_KEY);
  if (typeof savedCollapsed === 'boolean') {
    isFilterBarCollapsed = savedCollapsed;
  }
}

function toggleFilterTag(tag) {
  if (selectedFilterTags.has(tag)) {
    selectedFilterTags.delete(tag);
  } else {
    selectedFilterTags.add(tag);
  }
  persistFilterPreferences();
  showLessBtn.style.display = 'none';
  renderFilterBar();
  renderNotes();
}

function clearFilterTags() {
  if (selectedFilterTags.size === 0) {
    return;
  }
  selectedFilterTags.clear();
  persistFilterPreferences();
  showLessBtn.style.display = 'none';
  renderFilterBar();
  renderNotes();
}

function getFilteredNotes() {
  if (selectedFilterTags.size === 0) {
    return currentNotes;
  }

  return currentNotes.filter((note) => getEffectiveTags(note).some((tag) => selectedFilterTags.has(tag)));
}

function renderFilterBar() {
  filterOptions.innerHTML = '';
  getFilterableTags().forEach((tag) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'filter-tag';
    if (selectedFilterTags.has(tag)) {
      button.classList.add('filter-tag-active');
    }
    if (tag === DEFAULT_TAG) {
      button.classList.add('filter-tag-default');
    }
    button.textContent = tag;
    button.addEventListener('click', () => toggleFilterTag(tag));
    filterOptions.appendChild(button);
  });

  const activeCount = selectedFilterTags.size;
  filterCount.textContent = activeCount > 0 ? `${activeCount} selected` : 'All notes';
  filterCollapsedCount.textContent = String(activeCount);
  filterCollapsedCount.style.display = activeCount > 0 ? 'inline-flex' : 'none';
  clearFiltersBtn.style.display = 'inline-flex';
  clearFiltersBtn.disabled = activeCount === 0;
  clearFiltersBtn.classList.toggle('filter-clear-btn-active', activeCount > 0);
}

function updateFilterBarState() {
  notesLayout.classList.toggle('notes-layout-collapsed', isFilterBarCollapsed);
  filterBar.classList.toggle('filter-bar-collapsed', isFilterBarCollapsed);
  filterBarContent.style.display = isFilterBarCollapsed ? 'none' : 'block';
  toggleFilterBarBtn.textContent = isFilterBarCollapsed ? '›' : '‹';
  toggleFilterBarBtn.setAttribute('aria-label', isFilterBarCollapsed ? 'Expand filter bar' : 'Collapse filter bar');
  toggleFilterBarBtn.setAttribute('aria-expanded', String(!isFilterBarCollapsed));
}

function updateNoteFormState() {
  noteFormShell.classList.toggle('note-form-shell-collapsed', isNoteFormCollapsed);
  noteFormTrigger.setAttribute('aria-expanded', String(!isNoteFormCollapsed));
  noteFormTriggerIcon.textContent = isNoteFormCollapsed ? '+' : '−';
  noteFormHeading.textContent = 'New note';
  noteFormSubheading.textContent = isNoteFormCollapsed
    ? 'Open the editor when you want to write a note.'
    : 'Write markdown, code fences, and notes here.';
}

function expandNoteForm() {
  isNoteFormCollapsed = false;
  updateNoteFormState();
}

function collapseNoteForm() {
  isNoteFormCollapsed = true;
  updateNoteFormState();
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

  decorateCodeBlocks(container);
}

async function copyTextToClipboard(text) {
  await navigator.clipboard.writeText(text);
}

function showCopyButtonState(button, label, className = '') {
  button.textContent = label;
  button.classList.toggle('code-copy-button-success', className === 'code-copy-button-success');
  button.classList.toggle('code-copy-button-error', className === 'code-copy-button-error');
}

function decorateCodeBlocks(container) {
  container.querySelectorAll('pre code').forEach((block) => {
    const pre = block.parentElement;
    if (!pre || pre.querySelector('.code-copy-button')) {
      return;
    }

    pre.classList.add('code-copy-enabled');

    const copyButton = document.createElement('button');
    copyButton.type = 'button';
    copyButton.className = 'code-copy-button';
    copyButton.textContent = 'Copy';
    copyButton.setAttribute('aria-label', 'Copy code block');
    copyButton.addEventListener('click', async (event) => {
      event.stopPropagation();
      window.clearTimeout(copyButton._resetTimer);

      try {
        await copyTextToClipboard(block.textContent || '');
        showCopyButtonState(copyButton, 'Copied', 'code-copy-button-success');
      } catch {
        showCopyButtonState(copyButton, 'Failed', 'code-copy-button-error');
      }

      copyButton._resetTimer = window.setTimeout(() => {
        showCopyButtonState(copyButton, 'Copy');
      }, 1600);
    });

    pre.appendChild(copyButton);
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

function createTagList(note, options = {}) {
  const wrapper = document.createElement('div');
  wrapper.className = 'tag-list';

  const effectiveTags = getEffectiveTags(note);
  const actualTags = normalizeTags(note.tags);
  effectiveTags.forEach((tag) => {
    const removable = actualTags.includes(tag);
    const chip = createTagChip(tag, {
      isDefault: tag === DEFAULT_TAG,
      removable,
      onRemove: () => saveTagsForNote(note.id, actualTags.filter((value) => value !== tag))
    });

    if (options.interactive) {
      chip.classList.add('tag-chip-actionable');
      chip.tabIndex = 0;
      chip.setAttribute('role', 'button');
      chip.setAttribute('aria-label', `Edit tags for ${note.title}`);
      chip.addEventListener('click', (event) => {
        if (event.target.closest('.tag-chip-remove')) {
          return;
        }
        event.stopPropagation();
        options.onTagClick?.(tag, chip);
      });
      chip.addEventListener('keydown', (event) => {
        if (event.key !== 'Enter' && event.key !== ' ') {
          return;
        }
        event.preventDefault();
        event.stopPropagation();
        options.onTagClick?.(tag, chip);
      });
    }

    wrapper.appendChild(chip);
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

async function saveNote(note, id = null) {
  if (isAuthenticated) {
    const endpoint = id ? `/notes/${id}` : '/notes';
    const method = id ? 'PUT' : 'POST';
    const res = await fetch(endpoint, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(note)
    });
    if (res.status === 401) {
      handleAuthError();
      return false;
    }
    return res.ok;
  }

  const notes = getLocalNotes();
  const now = new Date().toISOString();

  if (id) {
    const idx = notes.findIndex((entry) => String(entry.id) === String(id));
    if (idx === -1) {
      return false;
    }
    notes[idx].title = note.title;
    notes[idx].content = note.content;
    notes[idx].tags = note.tags;
    notes[idx].updated_at = now;
  } else {
    notes.push({
      ...note,
      id: Date.now(),
      created_at: now,
      updated_at: now
    });
  }

  saveLocalNotes(notes);
  return true;
}

function buildNotePayload({ titleInput: titleField, contentInput: contentField, tagsRoot = tagOptions }) {
  return {
    title: titleField.value,
    content: contentField.value,
    tags: normalizeTags(getSelectedFormTags(tagsRoot))
  };
}

function resetCreateForm() {
  titleInput.value = '';
  contentInput.value = '';
  contentInput.classList.remove('has-content');
  setSelectedFormTags([]);
  collapseNoteForm();
}

function clearModalState() {
  modalEditNoteId = null;
  modalMode = null;
  modalBody.innerHTML = '';
}

function openModal(mode) {
  modalMode = mode;
  modalOverlay.classList.add('active');
  modalContent.style.display = 'block';
}

function closeModal() {
  modalOverlay.classList.remove('active');
  modalContent.style.display = 'none';
  clearModalState();
}

function createModalEditForm(note) {
  const wrapper = document.createElement('div');
  wrapper.className = 'modal-edit-shell';

  const title = document.createElement('h2');
  title.className = 'modal-title';
  title.textContent = `Edit ${note.title}`;

  const editForm = document.createElement('form');
  editForm.className = 'modal-edit-form';

  const modalTitleInput = document.createElement('input');
  modalTitleInput.type = 'text';
  modalTitleInput.required = true;
  modalTitleInput.value = note.title;
  modalTitleInput.placeholder = 'Title';

  const modalTagPicker = document.createElement('div');
  modalTagPicker.className = 'tag-picker';
  const modalTagHeader = document.createElement('div');
  modalTagHeader.className = 'tag-picker-header';
  const modalTagLabel = document.createElement('span');
  modalTagLabel.className = 'tag-picker-label';
  modalTagLabel.textContent = 'Tags';
  const modalTagHelp = document.createElement('span');
  modalTagHelp.className = 'tag-picker-help';
  modalTagHelp.textContent = 'No selection falls back to default.';
  const modalTagOptions = document.createElement('div');
  modalTagOptions.className = 'tag-picker-options';
  renderFormTagPicker(note.tags, modalTagOptions, 'modal-note-tags');
  modalTagHeader.appendChild(modalTagLabel);
  modalTagHeader.appendChild(modalTagHelp);
  modalTagPicker.appendChild(modalTagHeader);
  modalTagPicker.appendChild(modalTagOptions);

  const modalContentInput = document.createElement('textarea');
  modalContentInput.required = true;
  modalContentInput.placeholder = 'Write markdown, code fences, and notes here...';
  modalContentInput.value = note.content;
  toggleTextareaHeight(modalContentInput);
  modalContentInput.addEventListener('input', () => toggleTextareaHeight(modalContentInput));

  const help = document.createElement('p');
  help.className = 'form-help';
  help.textContent = 'Saved content stays as markdown source. Notes are rendered as preview when displayed.';

  const actions = document.createElement('div');
  actions.className = 'form-actions modal-edit-actions';
  const saveBtn = document.createElement('button');
  saveBtn.type = 'submit';
  saveBtn.className = 'btn-primary';
  saveBtn.textContent = 'Save';
  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className = 'btn-danger';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', closeModal);
  actions.appendChild(saveBtn);
  actions.appendChild(cancelBtn);

  editForm.appendChild(modalTitleInput);
  editForm.appendChild(modalTagPicker);
  editForm.appendChild(modalContentInput);
  editForm.appendChild(help);
  editForm.appendChild(actions);

  editForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    const ok = await saveNote(buildNotePayload({
      titleInput: modalTitleInput,
      contentInput: modalContentInput,
      tagsRoot: modalTagOptions
    }), note.id);
    if (!ok) {
      return;
    }
    closeModal();
    await fetchNotes();
  });

  wrapper.appendChild(title);
  wrapper.appendChild(editForm);
  return { wrapper, modalTitleInput };
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

  const closeAllTagEditors = () => {
    notesDiv.querySelectorAll('.note-tag-editor-host-open').forEach((host) => {
      host.innerHTML = '';
      host.classList.remove('note-tag-editor-host-open');
    });
  };

  const filteredNotes = getFilteredNotes();
  notesEmptyState.style.display = filteredNotes.length === 0 ? 'block' : 'none';

  filteredNotes.forEach((note, index) => {
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

    const actions = document.createElement('div');
    actions.className = 'note-actions';

    const content = document.createElement('div');
    content.className = 'note-content';

    const tagEditorHost = document.createElement('div');
    tagEditorHost.className = 'note-tag-editor-host';

    const closeTagEditor = () => {
      tagEditorHost.innerHTML = '';
      tagEditorHost.classList.remove('note-tag-editor-host-open');
    };

    const openTagEditor = () => {
      closeAllTagEditors();
      tagEditorHost.innerHTML = '';
      tagEditorHost.appendChild(createNoteTagEditor(note, closeTagEditor));
      tagEditorHost.classList.add('note-tag-editor-host-open');
    };

    const toggleTagEditor = () => {
      if (tagEditorHost.childElementCount > 0) {
        closeTagEditor();
        return;
      }
      openTagEditor();
    };

    const tags = createTagList(note, {
      interactive: true,
      onTagClick: () => openTagEditor()
    });

    header.addEventListener('click', (event) => {
      if (event.target.closest('button, a, input, label, .tag-chip')) {
        return;
      }
      const willCollapse = content.classList.contains('expanded');
      content.classList.toggle('expanded');
      if (willCollapse) {
        closeTagEditor();
      }
    });

    actions.appendChild(createActionButton('Edit', 'btn-note', () => {
      closeTagEditor();
      editNote(note.id);
    }));
    actions.appendChild(createActionButton('Tags', 'btn-note', toggleTagEditor));
    actions.appendChild(createActionButton('Delete', 'btn-note', async () => {
      closeTagEditor();
      await deleteNote(note.id);
    }));

    const body = createRenderedBody(note.content);

    const meta = document.createElement('div');
    meta.className = 'note-meta';

    const updated = document.createElement('small');
    updated.className = 'note-updated';
    updated.textContent = `Updated: ${new Date(note.updated_at).toLocaleString()}`;

    const expandBtn = createActionButton('Expand', 'btn-expand', () => {
      closeTagEditor();
      expandNote(note.id);
    });

    meta.appendChild(updated);
    meta.appendChild(expandBtn);

    titleWrap.appendChild(title);
    titleWrap.appendChild(tags);
    header.appendChild(titleWrap);
    header.appendChild(actions);
    content.appendChild(body);
    content.appendChild(meta);
    noteCard.appendChild(header);
    noteCard.appendChild(tagEditorHost);
    noteCard.appendChild(content);
    notesDiv.appendChild(noteCard);
  });

  showMoreBtn.style.display = filteredNotes.length > VISIBLE_COUNT ? 'block' : 'none';
  showLessBtn.style.display = 'none';
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  const ok = await saveNote(buildNotePayload({
    titleInput,
    contentInput
  }));
  if (!ok) {
    return;
  }
  resetCreateForm();
  await fetchNotes();
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

  if (String(modalEditNoteId) === String(id)) {
    closeModal();
  }
  fetchNotes();
}

function editNote(id) {
  const note = getNoteById(id);
  if (!note) {
    return;
  }

  modalEditNoteId = note.id;
  modalBody.innerHTML = '';
  const { wrapper, modalTitleInput } = createModalEditForm(note);
  modalBody.appendChild(wrapper);
  openModal('edit');
  modalTitleInput.focus();
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

  openModal('preview');
}

showMoreBtn.addEventListener('click', () => {
  notesDiv.querySelectorAll('.note').forEach((noteCard) => {
    noteCard.style.display = 'block';
  });
  showMoreBtn.style.display = 'none';
  showLessBtn.style.display = 'block';
});

showLessBtn.addEventListener('click', () => {
  const filteredNotes = getFilteredNotes();
  notesDiv.querySelectorAll('.note').forEach((noteCard, index) => {
    noteCard.style.display = index >= VISIBLE_COUNT ? 'none' : 'block';
  });
  showLessBtn.style.display = 'none';
  showMoreBtn.style.display = filteredNotes.length > VISIBLE_COUNT ? 'block' : 'none';
  window.scrollTo({ top: 0, behavior: 'smooth' });
});

toggleFilterBarBtn.addEventListener('click', () => {
  isFilterBarCollapsed = !isFilterBarCollapsed;
  persistFilterPreferences();
  updateFilterBarState();
});
clearFiltersBtn.addEventListener('click', clearFilterTags);

noteFormTrigger.addEventListener('click', () => {
  if (isNoteFormCollapsed) {
    expandNoteForm();
    titleInput.focus();
    return;
  }

  if (!titleInput.value.trim() && !contentInput.value.trim() && getSelectedFormTags().length === 0) {
    collapseNoteForm();
  }
});
contentInput.addEventListener('input', () => toggleTextareaHeight(contentInput));
resetCreateBtn.addEventListener('click', resetCreateForm);
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

restoreFilterPreferences();
renderFormTagPicker();
renderFilterBar();
updateFilterBarState();
updateNoteFormState();
updateAuthUI();
toggleTextareaHeight();
fetchNotes();
