type WorkflowItem = {
  uuid: string;
  created_at: string;
  updated_at: string;
  body: string;
  children: WorkflowItem[];
};

type ItemsResponse = {
  items: WorkflowItem[];
};

const statusEl = document.getElementById("outline-status");
const errorEl = document.getElementById("outline-error");
const rootEl = document.getElementById("outline-root");

function renderTree(items: WorkflowItem[]): HTMLElement {
  const list = document.createElement("ul");
  list.className = "outline";

  for (const item of items) {
    const listItem = document.createElement("li");

    const body = document.createElement("span");
    body.className = "item-body";
    body.textContent = item.body;
    body.title = `UUID: ${item.uuid}\nUpdated: ${item.updated_at}`;
    listItem.appendChild(body);

    if (item.children.length > 0) {
      listItem.appendChild(renderTree(item.children));
    }

    list.appendChild(listItem);
  }

  return list;
}

async function loadOutline(): Promise<void> {
  if (!statusEl || !errorEl || !rootEl) {
    return;
  }

  try {
    const response = await fetch("/api/items");
    if (!response.ok) {
      throw new Error(`request failed with status ${response.status}`);
    }

    const data = (await response.json()) as ItemsResponse;
    rootEl.replaceChildren(renderTree(data.items));
    statusEl.textContent = `${data.items.length} root items`;
    errorEl.hidden = true;
    errorEl.textContent = "";
  } catch (error) {
    statusEl.textContent = "Unable to load items.";
    errorEl.hidden = false;
    errorEl.textContent = String(error);
  }
}

void loadOutline();
