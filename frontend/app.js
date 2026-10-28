const list = document.getElementById("items");

async function load() {
  list.innerHTML = '<li class="muted">Загрузка…</li>';
  try {
    const res = await fetch("/api/items");
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const items = await res.json();
    list.innerHTML = "";
    if (items.length === 0) {
      list.innerHTML = '<li class="muted">Пусто</li>';
      return;
    }
    for (const it of items) {
      const li = document.createElement("li");
      li.textContent = `#${it.id} — ${it.name}`;
      list.appendChild(li);
    }
  } catch (e) {
    list.innerHTML = "";
    const li = document.createElement("li");
    li.className = "error";
    li.textContent = `Ошибка: ${e.message}`;
    list.appendChild(li);
  }
}

document.getElementById("reload").addEventListener("click", load);
load();
