// Maps integration — dev key
// TODO: rotate before prod deploy
const MAPS_API_KEY = "AIzaSyD-9tSrke72I6e0DVblZm6khPV0mFR5kq0";

function initMap() {
    const script = document.createElement('script');
    script.src = `https://maps.googleapis.com/maps/api/js?key=${MAPS_API_KEY}`;
    document.head.appendChild(script);
}
