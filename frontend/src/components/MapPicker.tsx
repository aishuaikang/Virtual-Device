import { Button } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import markerIcon2x from "leaflet/dist/images/marker-icon-2x.png";
import markerIcon from "leaflet/dist/images/marker-icon.png";
import markerShadow from "leaflet/dist/images/marker-shadow.png";
import { main } from "../../wailsjs/go/models";
import { SearchPlaces } from "../../wailsjs/go/main/App";
import { NumberField } from "./ConfigFields";
import { ClearableTextInput } from "./ClearableInput";
import Icon from "./Icon";
import Modal from "./Modal";
import ErrorMessage from "./ErrorMessage";

interface Props {
  lat: number;
  lng: number;
  onConfirm: (lat: number, lng: number) => void;
  onClose: () => void;
}

export default function MapPicker({ lat, lng, onConfirm, onClose }: Props) {
  const { t } = useTranslation();
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);
  const marker = useRef<L.Marker | null>(null);
  const zoom = useRef<L.Control.Zoom | null>(null);
  const request = useRef(0);
  const [pos, setPos] = useState({ lat, lng });
  const [query, setQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState("");
  const [noResults, setNoResults] = useState(false);
  const [tileError, setTileError] = useState(false);
  const [results, setResults] = useState<main.LocationSearchResult[]>([]);

  useEffect(() => {
    if (!container.current) return;
    const view = L.map(container.current, { zoomControl: false }).setView([lat, lng], 13);
    map.current = view;
    zoom.current = L.control.zoom().addTo(view);
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      attribution: "© OpenStreetMap",
      maxZoom: 19,
    })
      .addTo(view)
      .on("tileerror", () => setTileError(true));
    const pin = L.marker([lat, lng], {
      draggable: true,
      icon: L.icon({
        iconUrl: markerIcon,
        iconRetinaUrl: markerIcon2x,
        shadowUrl: markerShadow,
        iconSize: [25, 41],
        iconAnchor: [12, 41],
        shadowSize: [41, 41],
      }),
    }).addTo(view);
    marker.current = pin;
    pin.on("dragend", () => {
      const point = pin.getLatLng();
      setPos({ lat: point.lat, lng: point.lng });
    });
    view.on("click", (event: L.LeafletMouseEvent) => {
      pin.setLatLng(event.latlng);
      setPos({ lat: event.latlng.lat, lng: event.latlng.lng });
    });
    const frame = window.requestAnimationFrame(() => view.invalidateSize());
    return () => {
      request.current++;
      window.cancelAnimationFrame(frame);
      view.remove();
      map.current = null;
      zoom.current = null;
    };
  }, []);

  useEffect(() => {
    const controls = zoom.current?.getContainer();
    for (const [selector, key] of [[".leaflet-control-zoom-in", "map.zoomIn"], [".leaflet-control-zoom-out", "map.zoomOut"]]) {
      const button = controls?.querySelector(selector);
      button?.setAttribute("title", t(key));
      button?.setAttribute("aria-label", t(key));
    }
  }, [t]);

  const move = (next: { lat: number; lng: number }) => {
    setPos(next);
    if (Math.abs(next.lat) <= 90 && Math.abs(next.lng) <= 180) {
      marker.current?.setLatLng(next);
      map.current?.panTo(next);
    }
  };

  const search = async () => {
    if (!query.trim() || searching) return;
    const id = ++request.current;
    setSearching(true);
    setError("");
    setNoResults(false);
    try {
      const next = (await SearchPlaces(query.trim())) ?? [];
      if (id !== request.current) return;
      setResults(next);
      setNoResults(!next.length);
    } catch (value) {
      if (id === request.current) {
        setResults([]);
        setError(String(value));
      }
    } finally {
      if (id === request.current) setSearching(false);
    }
  };

  return (
    <Modal title={t("map.title")} onClose={onClose} wide>
      <div className="map-search">
        <ClearableTextInput
          aria-label={t("map.search")}
          value={query}
          onValueChange={setQuery}
          placeholder={t("map.search")}
          onKeyDown={(event) => {
            if (event.key === "Enter") void search();
          }}
        />
        <Button type="default"
          className="button"
          onClick={() => void search()}
          disabled={searching || !query.trim()}
          aria-label={t("map.search")}
        >
          <Icon name="search" />
          {searching ? t("map.searching") : t("map.find")}
        </Button>
      </div>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          onConfirm(pos.lat, pos.lng);
        }}
      >
        <div className="map-body">
          <div className="map-canvas" ref={container} />
          {(error || noResults || results.length > 0) && (
            <div className="map-results">
              {error && (
                <div className="inline-error" role="alert">
                  <ErrorMessage error={error} messageKey="errors.search" />
                </div>
              )}
              {noResults && <p role="status">{t("map.noResults")}</p>}
              {results.map((result, index) => (
                <Button type="default"
                  key={index}
                  htmlType="button"
                  onClick={() => {
                    move({ lat: result.lat, lng: result.lng });
                    map.current?.setZoom(15);
                    setResults([]);
                  }}
                >
                  <strong>{result.displayName}</strong>
                  <code>
                    {result.lat.toFixed(6)}, {result.lng.toFixed(6)}
                  </code>
                </Button>
              ))}
            </div>
          )}
        </div>
        {tileError && <p className="map-warning">{t("map.tileError")}</p>}
        <div className="form-grid map-coordinates">
          <NumberField
            label={t("config.lat")}
            value={pos.lat}
            onChange={(value) => move({ ...pos, lat: value })}
            min={-90}
            max={90}
            step="any"
          />
          <NumberField
            label={t("config.lng")}
            value={pos.lng}
            onChange={(value) => move({ ...pos, lng: value })}
            min={-180}
            max={180}
            step="any"
          />
        </div>
        <footer className="dialog-actions">
          <Button type="default" htmlType="button" className="button" onClick={onClose}>
            {t("app.cancel")}
          </Button>
          <Button type="primary" htmlType="submit" className="button button-primary">{t("app.confirm")}</Button>
        </footer>
      </form>
    </Modal>
  );
}
