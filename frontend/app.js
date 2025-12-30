const dom = {
  apiBase: document.getElementById("api-base"),
  apiSave: document.getElementById("api-save"),
  apiBaseLabel: document.getElementById("api-base-label"),
  apiStatus: document.getElementById("api-status"),
  apiDot: document.getElementById("api-dot"),
  healthBtn: document.getElementById("health-btn"),
  refreshBtn: document.getElementById("refresh-btn"),
  notice: document.getElementById("notice"),
  createForm: document.getElementById("create-form"),
  createTitle: document.getElementById("create-title"),
  createTags: document.getElementById("create-tags"),
  createDone: document.getElementById("create-done"),
  filterForm: document.getElementById("filter-form"),
  filterDone: document.getElementById("filter-done"),
  filterTag: document.getElementById("filter-tag"),
  filterClear: document.getElementById("filter-clear"),
  updateForm: document.getElementById("update-form"),
  updateId: document.getElementById("update-id"),
  updateTitle: document.getElementById("update-title"),
  updateTags: document.getElementById("update-tags"),
  updateDone: document.getElementById("update-done"),
  clearTags: document.getElementById("clear-tags"),
  fetchBtn: document.getElementById("fetch-btn"),
  deleteBtn: document.getElementById("delete-btn"),
  tasksList: document.getElementById("tasks-list"),
  tasksCount: document.getElementById("tasks-count"),
  details: document.getElementById("details"),
  refreshList: document.getElementById("refresh-list"),
};

const state = {
  baseUrl:
    localStorage.getItem("apiBaseUrl") ||
    window.API_BASE_URL ||
    "/api",
  tasks: [],
  selected: null,
  filters: {
    done: "all",
    tag: "",
  },
};

function setApiBase(value, persist = true) {
  const trimmed = value.replace(/\/+$/, "");
  state.baseUrl = trimmed;
  dom.apiBase.value = trimmed;
  dom.apiBaseLabel.textContent = trimmed;
  if (persist) {
    localStorage.setItem("apiBaseUrl", trimmed);
  }
}

function parseTags(input) {
  return input
    .split(",")
    .map((tag) => tag.trim())
    .filter((tag) => tag.length > 0);
}

function showNotice(type, message) {
  dom.notice.textContent = message;
  dom.notice.className = `notice show ${type}`;
  clearTimeout(showNotice.timer);
  showNotice.timer = setTimeout(() => {
    dom.notice.className = "notice";
  }, 3200);
}

function setStatus(status, detail) {
  dom.apiStatus.textContent = status;
  dom.apiDot.className = "dot";
  if (detail === "ok") {
    dom.apiDot.classList.add("ok");
  }
  if (detail === "error") {
    dom.apiDot.classList.add("error");
  }
}

function formatDate(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("it-IT", {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

async function apiRequest(path, options = {}) {
  const url = `${state.baseUrl}${path}`;
  const headers = {
    Accept: "application/json",
    ...(options.headers || {}),
  };

  const response = await fetch(url, {
    ...options,
    headers,
  });

  if (response.status === 204) {
    return null;
  }

  const contentType = response.headers.get("content-type") || "";
  const payload = contentType.includes("application/json")
    ? await response.json()
    : await response.text();

  if (!response.ok) {
    const detail =
      (payload && payload.detail) ||
      (payload && payload.message) ||
      response.statusText ||
      "Errore inatteso";
    throw new Error(detail);
  }

  return payload;
}

async function checkHealth() {
  try {
    const data = await apiRequest("/health");
    setStatus("Online", "ok");
    showNotice("success", `API OK • Mongo: ${data.mongo}`);
  } catch (error) {
    setStatus("Offline", "error");
    showNotice("error", `Health check fallito: ${error.message}`);
  }
}

async function loadTasks() {
  dom.tasksList.innerHTML = `<div class="task-meta">Caricamento...</div>`;
  try {
    const params = new URLSearchParams();
    if (state.filters.done !== "all") {
      params.set("done", state.filters.done === "done");
    }
    if (state.filters.tag) {
      params.set("tag", state.filters.tag);
    }
    const path = params.toString() ? `/tasks?${params}` : "/tasks";
    const data = await apiRequest(path);
    state.tasks = data.items || [];
    dom.tasksCount.textContent = data.count ?? state.tasks.length;
    renderTasks();
  } catch (error) {
    dom.tasksList.innerHTML = "";
    showNotice("error", `Errore lista: ${error.message}`);
  }
}

function renderTasks() {
  dom.tasksList.innerHTML = "";
  if (!state.tasks.length) {
    dom.tasksList.innerHTML = `<div class="task-meta">Nessuna task trovata.</div>`;
    return;
  }

  state.tasks.forEach((task) => {
    const card = document.createElement("article");
    card.className = `task${task.done ? " done" : ""}`;
    card.dataset.id = task.id;

    const header = document.createElement("div");
    header.className = "task-header";

    const info = document.createElement("div");
    const title = document.createElement("h3");
    title.textContent = task.title;
    const meta = document.createElement("div");
    meta.className = "task-meta";
    meta.textContent = `ID: ${task.id}`;

    const time = document.createElement("div");
    time.className = "task-meta";
    time.textContent = `Creato: ${formatDate(task.createdAt)}`;

    info.appendChild(title);
    info.appendChild(meta);
    info.appendChild(time);

    const actions = document.createElement("div");
    actions.className = "task-actions";
    actions.appendChild(makeAction("view", "Dettagli", "ghost"));
    actions.appendChild(
      makeAction("toggle", task.done ? "Riapri" : "Completa", "primary")
    );
    actions.appendChild(makeAction("delete", "Elimina", "danger"));

    header.appendChild(info);
    header.appendChild(actions);

    const tags = document.createElement("div");
    tags.className = "tags";
    if (Array.isArray(task.tags) && task.tags.length) {
      task.tags.forEach((tag) => {
        const badge = document.createElement("span");
        badge.className = "tag";
        badge.textContent = tag;
        tags.appendChild(badge);
      });
    } else {
      const badge = document.createElement("span");
      badge.className = "tag";
      badge.textContent = "no-tags";
      tags.appendChild(badge);
    }

    card.appendChild(header);
    card.appendChild(tags);
    dom.tasksList.appendChild(card);
  });
}

function makeAction(action, text, variant) {
  const button = document.createElement("button");
  button.dataset.action = action;
  button.textContent = text;
  if (variant) {
    button.classList.add(variant);
  }
  return button;
}

async function createTask(event) {
  event.preventDefault();
  const title = dom.createTitle.value.trim();
  if (!title || title.length < 3) {
    showNotice("error", "Titolo troppo corto.");
    return;
  }

  const payload = {
    title,
    done: dom.createDone.checked,
  };

  const tags = parseTags(dom.createTags.value);
  if (tags.length) {
    payload.tags = tags;
  }

  try {
    await apiRequest("/tasks", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    dom.createForm.reset();
    showNotice("success", "Task creata.");
    await loadTasks();
  } catch (error) {
    showNotice("error", `Creazione fallita: ${error.message}`);
  }
}

async function updateTask(event) {
  event.preventDefault();
  const id = dom.updateId.value.trim();
  if (!id) {
    showNotice("error", "Inserisci un ID.");
    return;
  }

  const payload = {};
  const title = dom.updateTitle.value.trim();
  if (title) {
    payload.title = title;
  }

  if (dom.clearTags.checked) {
    payload.tags = [];
  } else {
    const tags = parseTags(dom.updateTags.value);
    if (tags.length) {
      payload.tags = tags;
    }
  }

  if (dom.updateDone.value !== "") {
    payload.done = dom.updateDone.value === "true";
  }

  if (!Object.keys(payload).length) {
    showNotice("error", "Nessun campo da aggiornare.");
    return;
  }

  try {
    await apiRequest(`/tasks/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    showNotice("success", "Task aggiornata.");
    await loadTasks();
    await loadTaskById(id);
  } catch (error) {
    showNotice("error", `Update fallito: ${error.message}`);
  }
}

async function loadTaskById(id) {
  try {
    const data = await apiRequest(`/tasks/${id}`);
    state.selected = data;
    renderDetails();
    fillUpdateForm(data);
    showNotice("success", "Task caricata.");
  } catch (error) {
    showNotice("error", `Fetch fallito: ${error.message}`);
  }
}

function fillUpdateForm(task) {
  dom.updateId.value = task.id || "";
  dom.updateTitle.value = task.title || "";
  dom.updateTags.value = Array.isArray(task.tags) ? task.tags.join(", ") : "";
  dom.updateDone.value = task.done ? "true" : "false";
  dom.clearTags.checked = false;
}

function renderDetails() {
  dom.details.innerHTML = "";
  if (!state.selected) {
    dom.details.innerHTML = `<div class="details-empty">Nessuna task selezionata.</div>`;
    return;
  }

  const card = document.createElement("div");
  card.className = "details-card";

  const title = document.createElement("h3");
  title.textContent = state.selected.title;

  const list = document.createElement("div");
  list.className = "details-list";

  list.appendChild(buildDetail("ID", state.selected.id, true));
  list.appendChild(buildDetail("Stato", state.selected.done ? "Completata" : "Aperta"));
  list.appendChild(buildDetail("Creato", formatDate(state.selected.createdAt)));
  list.appendChild(
    buildDetail(
      "Tag",
      Array.isArray(state.selected.tags) && state.selected.tags.length
        ? state.selected.tags.join(", ")
        : "Nessun tag"
    )
  );

  card.appendChild(title);
  card.appendChild(list);
  dom.details.appendChild(card);
}

function buildDetail(label, value, mono = false) {
  const row = document.createElement("div");
  const labelSpan = document.createElement("span");
  labelSpan.textContent = `${label}:`;
  const valueSpan = document.createElement("div");
  valueSpan.textContent = value;
  if (mono) {
    valueSpan.className = "mono";
  }
  row.appendChild(labelSpan);
  row.appendChild(valueSpan);
  return row;
}

async function deleteTask() {
  const id = dom.updateId.value.trim();
  if (!id) {
    showNotice("error", "Inserisci un ID per eliminare.");
    return;
  }

  const confirmed = window.confirm("Confermi eliminazione task?");
  if (!confirmed) return;

  try {
    await apiRequest(`/tasks/${id}`, { method: "DELETE" });
    showNotice("success", "Task eliminata.");
    state.selected = null;
    dom.updateForm.reset();
    renderDetails();
    await loadTasks();
  } catch (error) {
    showNotice("error", `Delete fallito: ${error.message}`);
  }
}

async function toggleTask(id) {
  const task = state.tasks.find((item) => item.id === id);
  if (!task) return;
  try {
    await apiRequest(`/tasks/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ done: !task.done }),
    });
    showNotice("success", "Stato aggiornato.");
    await loadTasks();
    if (state.selected && state.selected.id === id) {
      await loadTaskById(id);
    }
  } catch (error) {
    showNotice("error", `Toggle fallito: ${error.message}`);
  }
}

dom.apiSave.addEventListener("click", () => {
  if (!dom.apiBase.value.trim()) {
    showNotice("error", "Inserisci una base URL valida.");
    return;
  }
  setApiBase(dom.apiBase.value);
  checkHealth();
  loadTasks();
});

dom.healthBtn.addEventListener("click", checkHealth);
dom.refreshBtn.addEventListener("click", loadTasks);
dom.refreshList.addEventListener("click", loadTasks);

dom.createForm.addEventListener("submit", createTask);
dom.filterForm.addEventListener("submit", (event) => {
  event.preventDefault();
  state.filters.done = dom.filterDone.value;
  state.filters.tag = dom.filterTag.value.trim();
  loadTasks();
});
dom.filterClear.addEventListener("click", () => {
  dom.filterForm.reset();
  state.filters.done = "all";
  state.filters.tag = "";
  loadTasks();
});

dom.updateForm.addEventListener("submit", updateTask);
dom.fetchBtn.addEventListener("click", () => {
  const id = dom.updateId.value.trim();
  if (!id) {
    showNotice("error", "Inserisci un ID.");
    return;
  }
  loadTaskById(id);
});
dom.deleteBtn.addEventListener("click", deleteTask);

dom.tasksList.addEventListener("click", (event) => {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  const card = event.target.closest(".task");
  if (!card) return;
  const id = card.dataset.id;
  const action = button.dataset.action;

  if (action === "view") {
    loadTaskById(id);
  }
  if (action === "toggle") {
    toggleTask(id);
  }
  if (action === "delete") {
    dom.updateId.value = id;
    deleteTask();
  }
});

setApiBase(state.baseUrl, false);
checkHealth();
loadTasks();
