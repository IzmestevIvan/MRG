// app.js — клиентская логика карт на Leaflet/OpenStreetMap.
// Карты и геокодинг (Nominatim) выполняются в браузере пользователя,
// серверу внешние сервисы не нужны.
(function () {
  "use strict";

  var OSM_URL = "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png";
  var OSM_ATTR = "© OpenStreetMap";
  var RUSSIA_CENTER = [61.5, 105.0];

  function tileLayer() {
    return L.tileLayer(OSM_URL, { maxZoom: 19, attribution: OSM_ATTR });
  }

  // buildingURL повторяет серверную логику ссылки на страницу дома.
  function buildingURL(a) {
    var p = new URLSearchParams({
      city: a.city,
      street: a.street,
      house: a.house,
      lat: Number(a.lat).toFixed(6),
      lon: Number(a.lon).toFixed(6),
    });
    return "/building?" + p.toString();
  }

  // initHomeMap — карта России с маркерами домов, у которых есть отзывы.
  function initHomeMap(el) {
    var map = L.map(el).setView(RUSSIA_CENTER, 3);
    tileLayer().addTo(map);

    fetch(el.dataset.api)
      .then(function (r) { return r.json(); })
      .then(function (items) {
        (items || []).forEach(function (b) {
          L.marker([b.lat, b.lon])
            .addTo(map)
            .bindPopup(
              '<b>' + escapeHtml(b.title) + '</b><br>' +
              '★ ' + b.avg.toFixed(1) + ' · ' + b.reviews + ' отз.<br>' +
              '<a href="' + b.url + '">Открыть дом →</a>'
            );
        });
      })
      .catch(function () { /* карта остаётся пустой при сбое сети */ });

    return map;
  }

  // initSearch — поиск адреса через Nominatim и переход на страницу дома.
  function initSearch(form, input, results, map) {
    var searchMarker = null;

    form.addEventListener("submit", function (e) {
      e.preventDefault();
      var q = input.value.trim();
      if (!q) return;
      results.textContent = "Ищем…";

      var url = "https://nominatim.openstreetmap.org/search?format=json" +
        "&addressdetails=1&countrycodes=ru&limit=5&q=" + encodeURIComponent(q);

      fetch(url, { headers: { "Accept-Language": "ru" } })
        .then(function (r) { return r.json(); })
        .then(function (found) { renderResults(found); })
        .catch(function () { results.textContent = "Не удалось выполнить поиск."; });
    });

    function renderResults(found) {
      results.innerHTML = "";
      if (!found || !found.length) {
        results.textContent = "Ничего не найдено.";
        return;
      }
      found.forEach(function (item) {
        var addr = toAddress(item);
        if (!addr) return;
        var btn = document.createElement("button");
        btn.type = "button";
        btn.className = "result";
        btn.textContent = item.display_name;
        btn.addEventListener("click", function () { choose(addr); });
        results.appendChild(btn);
      });
      if (!results.children.length) {
        results.textContent = "В результатах нет точного адреса с номером дома.";
      }
    }

    function choose(addr) {
      if (map) {
        map.setView([addr.lat, addr.lon], 17);
        if (searchMarker) map.removeLayer(searchMarker);
        searchMarker = L.marker([addr.lat, addr.lon]).addTo(map);
      }
      window.location.href = buildingURL(addr);
    }
  }

  // toAddress извлекает из ответа Nominatim город, улицу и дом.
  function toAddress(item) {
    var a = item.address || {};
    var city = a.city || a.town || a.village || a.municipality || a.county;
    var street = a.road;
    var house = a.house_number;
    if (!city || !street || !house) return null;
    return {
      city: city,
      street: street,
      house: house,
      lat: parseFloat(item.lat),
      lon: parseFloat(item.lon),
    };
  }

  // initBuildingMap — мини-карта с маркером конкретного дома.
  function initBuildingMap(el) {
    var lat = parseFloat(el.dataset.lat);
    var lon = parseFloat(el.dataset.lon);
    var map = L.map(el).setView([lat, lon], 17);
    tileLayer().addTo(map);
    L.marker([lat, lon]).addTo(map);
    return map;
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    var home = document.getElementById("map");
    if (home) {
      var map = initHomeMap(home);
      var form = document.getElementById("search-form");
      var input = document.getElementById("search-input");
      var results = document.getElementById("search-results");
      if (form && input && results) initSearch(form, input, results, map);
    }
    var bmap = document.getElementById("bmap");
    if (bmap) initBuildingMap(bmap);
  });
})();
