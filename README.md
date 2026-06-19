# Phone Dashboard

Dashboard web léger pour surveiller un téléphone sous **postmarketOS** (ou tout
appareil Linux) : CPU, RAM, espace disque, températures et batterie — en valeur
actuelle et en graphiques sur 7 jours.

Conçu pour une **conso minimale** : un seul binaire Go (lecture directe de
`/proc` et `/sys`), SQLite embarqué, et le frontend Vue servi par le même binaire.
Pas de serveur de base de données séparé, pas de nginx.

## Stack

- **Backend** : Go (`gopsutil` + lecture sysfs), SQLite pur-Go (`modernc.org/sqlite`).
- **Frontend** : Vue 3 + Vite + Tailwind + Chart.js, embarqué dans le binaire.
- **Auth** : compte admin créé au premier lancement (bcrypt + cookie de session JWT).
- **Déploiement** : un seul conteneur Docker.

## Fonctionnalités

- Métriques temps réel : CPU %, RAM (% + Mo), disque (% + libre), batterie
  (% + état de charge), températures (un capteur par carte).
- Graphiques historiques : Live / 1h / 24h / 7 jours (agrégation à la volée).
- **Fréquence de collecte configurable** depuis l'UI (5 s par défaut), appliquée à chaud.
- **Rétention configurable** (7 jours par défaut), purge automatique.
- **Premier lancement** : un panneau demande les identifiants admin, les stocke en
  base, puis ne réapparaît plus jamais.

## Démarrage rapide (Docker Compose)

```bash
docker compose up --build -d
```

Puis ouvrez `http://<ip-du-téléphone>:8080`. Le premier écran crée le compte admin.

### Montages requis

Pour lire les métriques **de l'hôte** (et non du conteneur), le compose monte en
lecture seule :

- `/proc` → `/host/proc` (CPU, RAM ; via `HOST_PROC`)
- `/sys` → `/host/sys` (batterie, températures ; via `HOST_SYS`)

La batterie est lue dans `/sys/class/power_supply/*` et les températures dans
`/sys/class/thermal/thermal_zone*` + `/sys/class/hwmon/*`. Sur une machine x86 sans
batterie, la carte batterie est simplement masquée (dégradation gracieuse).

## Déploiement via Dokploy

1. Créez une application de type **Docker Compose** pointant sur ce dépôt.
2. Dokploy construit l'image nativement (arm64 sur le téléphone) et expose le
   service. Associez votre domaine au port **8080** ; Dokploy/Traefik gère le TLS.
3. Le cookie de session est marqué `Secure` automatiquement dès que la requête
   arrive en HTTPS (`X-Forwarded-Proto`), donc rien à configurer côté HTTPS.

## Variables d'environnement

| Variable    | Défaut                 | Rôle                                  |
|-------------|------------------------|---------------------------------------|
| `PORT`      | `8080`                 | Port d'écoute HTTP                    |
| `DB_PATH`   | `/data/dashboard.db`   | Fichier SQLite (volume persistant)    |
| `HOST_PROC` | `/host/proc`           | Racine `/proc` de l'hôte (gopsutil)   |
| `HOST_SYS`  | `/host/sys`            | Racine `/sys` de l'hôte               |
| `DISK_PATH` | `/`                    | Point de montage dont on lit l'usage  |

## Développement local

```bash
# Backend (nécessite Go 1.23+)
cd backend && go run .

# Frontend (proxy /api -> :8080)
cd frontend && npm install && npm run dev
```

## API

| Méthode | Route                          | Auth | Rôle                          |
|---------|--------------------------------|------|-------------------------------|
| GET     | `/api/status`                  | non  | état setup + session          |
| POST    | `/api/setup`                   | non* | crée l'admin (1ʳᵉ fois)       |
| POST    | `/api/login`                   | non  | connexion                     |
| POST    | `/api/logout`                  | oui  | déconnexion                   |
| GET     | `/api/metrics/current`         | oui  | dernière mesure               |
| GET     | `/api/metrics/history?range=`  | oui  | série agrégée (live/1h/24h/7d)|
| GET     | `/api/settings`                | oui  | réglages actuels              |
| PUT     | `/api/settings`                | oui  | met à jour collecte/rétention |

\* `/api/setup` est verrouillé dès qu'un admin existe.
