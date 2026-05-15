document.addEventListener("DOMContentLoaded", () => {
  const statusColors = {
    placed: { text: "text-pos_plc", bg: "bg-pos_plc" },
    transit: { text: "text-pos_trs", bg: "bg-pos_trs" },
    ready: { text: "text-pos_rdy", bg: "bg-pos_rdy" },
    preparing: { text: "text-pos_pdg", bg: "bg-pos_pdg" },
    canceled: { text: "text-pos_cld", bg: "bg-pos_cld" },
    taken: { text: "text-pos_dlv", bg: "bg-pos_dlv" },
    served: { text: "text-pos_dlv", bg: "bg-pos_dlv" },
    delivered: { text: "text-pos_dlv", bg: "bg-pos_dlv" },
  };

  document.querySelectorAll(".status").forEach((statusSpan) => {
    const status = statusSpan.textContent.trim().toLowerCase();
    const color = statusColors[status];
    if (!color) return;

    const card = statusSpan.closest(".snap-center");
    if (!card) return;

    // Apply color to .status
    statusSpan.classList.add(color.text);

    // Apply color to all .scolor elements inside the same card
    card.querySelectorAll(".scolor").forEach((el) => {
      el.classList.add(color.text);
    });

    // Apply bg to .sback inside the card
    card.querySelectorAll(".sback").forEach((el) => {
      el.classList.add(color.bg);
    });
  });

  const typeIcons = {
    takeaway: "hgi-packaging",
    dinein: "hgi-spoon-and-fork",
    delivery: "hgi-scooter-02",
  };

  document.querySelectorAll(".snap-center").forEach((card) => {
    const typeSpan = card.querySelector(".otype");
    if (!typeSpan) return;

    const type = typeSpan.textContent.trim().toLowerCase();
    const iconClass = typeIcons[type];
    if (!iconClass) return;

    const icon = card.querySelector(".otype-icon");
    if (icon) icon.classList.add(iconClass);
  });
});

// changing order status after print
function handlePrint(orderId, orderType) {
  window.print();

  const safeId = encodeURIComponent(orderId);

  setTimeout(() => {
    htmx.ajax('POST', `/pos/order/update-status/${safeId}`, {
      values: { type: orderType },
      target: '#receipt'
    });
  }, 5000);
}

