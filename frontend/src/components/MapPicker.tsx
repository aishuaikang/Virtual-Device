import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import markerIcon2x from "leaflet/dist/images/marker-icon-2x.png";
import markerIcon from "leaflet/dist/images/marker-icon.png";
import markerShadow from "leaflet/dist/images/marker-shadow.png";
import { main } from "../../wailsjs/go/models";
import { SearchPlaces } from "../../wailsjs/go/main/App";
import { ClearableNumberInput, ClearableTextInput } from "./ClearableInput";

// Fix default marker icon paths broken by bundlers
delete (L.Icon.Default.prototype as any)._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: markerIcon2x,
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
});

interface Props {
  lat: number;
  lng: number;
  onConfirm: (lat: number, lng: number) => void;
  onClose: () => void;
}

export default function MapPicker({ lat, lng, onConfirm, onClose }: Props) {
  const { t } = useTranslation();
  const mapRef = useRef<HTMLDivElement>(null);
  const mapInstanceRef = useRef<L.Map | null>(null);
  const markerRef = useRef<L.Marker | null>(null);
  const [pos, setPos] = useState<{ lat: number; lng: number }>({ lat, lng });
  const [query, setQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState("");
  const [tileError, setTileError] = useState(false);
  const [results, setResults] = useState<main.LocationSearchResult[]>([]);
  const showSearchPanel = searchError !== "" || results.length > 0;

  useEffect(() => {
    if (!mapRef.current || mapInstanceRef.current) return;

    const map = L.map(mapRef.current).setView([lat, lng], 13);
    mapInstanceRef.current = map;

    const tileLayer = L.tileLayer(
      "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
      {
      attribution: "© OpenStreetMap contributors",
      maxZoom: 19,
      },
    ).addTo(map);
    tileLayer.on("tileerror", () => {
      setTileError(true);
    });

    const marker = L.marker([lat, lng], { draggable: true }).addTo(map);
    markerRef.current = marker;

    marker.on("dragend", () => {
      const { lat: la, lng: ln } = marker.getLatLng();
      setPos({ lat: la, lng: ln });
    });

    map.on("click", (e: L.LeafletMouseEvent) => {
      const { lat: la, lng: ln } = e.latlng;
      marker.setLatLng([la, ln]);
      setPos({ lat: la, lng: ln });
    });

    return () => {
      map.remove();
      mapInstanceRef.current = null;
    };
  }, []);

  const updateLat = (nextLat: number) => {
    setPos((current) => {
      markerRef.current?.setLatLng([nextLat, current.lng]);
      mapInstanceRef.current?.setView([nextLat, current.lng]);
      return { ...current, lat: nextLat };
    });
  };

  const updateLng = (nextLng: number) => {
    setPos((current) => {
      markerRef.current?.setLatLng([current.lat, nextLng]);
      mapInstanceRef.current?.setView([current.lat, nextLng]);
      return { ...current, lng: nextLng };
    });
  };

  const moveTo = (latValue: number, lngValue: number, zoom = 15) => {
    setSearchError("");
    setResults([]);
    setPos({ lat: latValue, lng: lngValue });
    markerRef.current?.setLatLng([latValue, lngValue]);
    mapInstanceRef.current?.setView([latValue, lngValue], zoom);
  };

  const handleSearch = async () => {
    if (!query.trim() || searching) return;

    setSearching(true);
    setSearchError("");
    try {
      const nextResults = await SearchPlaces(query.trim());
      setResults(nextResults ?? []);
      if (!nextResults || nextResults.length === 0) {
        setSearchError(t("map.searchNoResult"));
      }
    } catch (error) {
      console.error("Failed to search places:", error);
      setResults([]);
      setSearchError(`${t("map.searchFailed")} ${t("map.networkHint")}`);
    } finally {
      setSearching(false);
    }
  };

  return (
    <div
      className="map-picker-overlay"
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <div className="map-picker-modal">
        <div className="map-picker-header">
          <div className="map-picker-title-group">
            <span className="map-picker-title">{t("map.title")}</span>
            <span className="map-picker-subtitle">{t("map.clickHint")}</span>
          </div>
          <button className="map-picker-close" onClick={onClose}>
            ✕
          </button>
        </div>
        <div className="map-picker-toolbar">
          <div className="map-picker-section-card map-picker-search-card">
            <div className="map-picker-section-label">{t("map.search")}</div>
            <div className="map-picker-search">
              <ClearableTextInput
                containerClassName="map-picker-search-input"
                className="map-picker-search-field"
                value={query}
                onValueChange={setQuery}
                placeholder={t("map.searchPlaceholder")}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    void handleSearch();
                  }
                }}
              />
              <button
                className="btn btn-secondary map-picker-search-btn"
                disabled={searching || query.trim() === ""}
                onClick={() => void handleSearch()}
              >
                {searching ? t("map.searching") : t("map.search")}
              </button>
            </div>
          </div>
          <div className="map-picker-section-card map-picker-coords-card">
            <div className="map-picker-section-label">
              {t("config.lat")} / {t("config.lng")}
            </div>
            <div className="map-picker-coords">
              <label className="map-picker-coord-field">
                <span className="map-picker-field-label">
                  {t("config.lat")}
                </span>
                <ClearableNumberInput
                  step="0.000001"
                  placeholder={t("config.lat")}
                  value={pos.lat}
                  onValueChange={updateLat}
                />
              </label>
              <label className="map-picker-coord-field">
                <span className="map-picker-field-label">
                  {t("config.lng")}
                </span>
                <ClearableNumberInput
                  step="0.000001"
                  placeholder={t("config.lng")}
                  value={pos.lng}
                  onValueChange={updateLng}
                />
              </label>
            </div>
          </div>
        </div>
        <div
          className={`map-picker-body${showSearchPanel ? " map-picker-body-with-panel" : ""}`}
        >
          {showSearchPanel && (
            <aside className="map-picker-results-panel">
              <div className="map-picker-results-title">{t("map.search")}</div>
              {searchError && (
                <div className="map-picker-search-status">{searchError}</div>
              )}
              {results.length > 0 && (
                <ul className="map-picker-results">
                  {results.map((result, index) => (
                    <li key={`${result.displayName}-${index}`}>
                      <button
                        type="button"
                        className="map-picker-result-btn"
                        onClick={() => moveTo(result.lat, result.lng)}
                      >
                        <span className="map-picker-result-name">
                          {result.displayName}
                        </span>
                        <span className="map-picker-result-coords">
                          {result.lat.toFixed(6)}, {result.lng.toFixed(6)}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </aside>
          )}
          <div className="map-picker-map-shell">
            {tileError && (
              <div className="map-picker-map-alert">
                {t("map.tileLoadFailed")}
              </div>
            )}
            <div className="map-picker-map" ref={mapRef} />
          </div>
        </div>
        <div className="map-picker-footer">
          <span className="map-picker-hint">
            {tileError ? t("map.networkHint") : t("map.clickHint")}
          </span>
          <div className="map-picker-actions">
            <button className="btn btn-secondary" onClick={onClose}>
              {t("app.cancel")}
            </button>
            <button
              className="btn btn-primary"
              onClick={() => onConfirm(pos.lat, pos.lng)}
            >
              {t("app.confirm")}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
