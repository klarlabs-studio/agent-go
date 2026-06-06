// Package geo provides geolocation utilities for agents.
package geo

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the geolocation tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("geo").
		WithDescription("Geolocation utilities").
		AddTools(
			distanceTool(),
			bearingTool(),
			destinationTool(),
			midpointTool(),
			boundingBoxTool(),
			parseCoordTool(),
			formatCoordTool(),
			validateCoordTool(),
			convertCoordTool(),
			areaPolygonTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

const (
	earthRadiusKm    = 6371.0
	earthRadiusMiles = 3959.0
)

func toRadians(deg float64) float64 {
	return deg * math.Pi / 180
}

func toDegrees(rad float64) float64 {
	return rad * 180 / math.Pi
}

func distanceTool() tool.Tool {
	return tool.NewBuilder("geo_distance").
		WithDescription("Calculate distance between two coordinates").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat1 float64 `json:"lat1"`
				Lon1 float64 `json:"lon1"`
				Lat2 float64 `json:"lat2"`
				Lon2 float64 `json:"lon2"`
				Unit string  `json:"unit,omitempty"` // km (default), miles, m, nm
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Haversine formula
			lat1Rad := toRadians(params.Lat1)
			lat2Rad := toRadians(params.Lat2)
			deltaLat := toRadians(params.Lat2 - params.Lat1)
			deltaLon := toRadians(params.Lon2 - params.Lon1)

			a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
				math.Cos(lat1Rad)*math.Cos(lat2Rad)*
					math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
			c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

			distanceKm := earthRadiusKm * c

			var distance float64
			unit := strings.ToLower(params.Unit)
			switch unit {
			case "miles", "mi":
				distance = distanceKm * 0.621371
				unit = "miles"
			case "m", "meters":
				distance = distanceKm * 1000
				unit = "m"
			case "nm", "nautical":
				distance = distanceKm * 0.539957
				unit = "nm"
			default:
				distance = distanceKm
				unit = "km"
			}

			result := map[string]any{
				"distance": distance,
				"unit":     unit,
				"from": map[string]float64{
					"lat": params.Lat1,
					"lon": params.Lon1,
				},
				"to": map[string]float64{
					"lat": params.Lat2,
					"lon": params.Lon2,
				},
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bearingTool() tool.Tool {
	return tool.NewBuilder("geo_bearing").
		WithDescription("Calculate bearing between two coordinates").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat1 float64 `json:"lat1"`
				Lon1 float64 `json:"lon1"`
				Lat2 float64 `json:"lat2"`
				Lon2 float64 `json:"lon2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			lat1Rad := toRadians(params.Lat1)
			lat2Rad := toRadians(params.Lat2)
			deltaLon := toRadians(params.Lon2 - params.Lon1)

			y := math.Sin(deltaLon) * math.Cos(lat2Rad)
			x := math.Cos(lat1Rad)*math.Sin(lat2Rad) -
				math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(deltaLon)

			bearing := toDegrees(math.Atan2(y, x))
			bearing = math.Mod(bearing+360, 360) // Normalize to 0-360

			// Compass direction
			directions := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
			idx := int((bearing+22.5)/45) % 8
			compass := directions[idx]

			result := map[string]any{
				"bearing": bearing,
				"compass": compass,
				"from": map[string]float64{
					"lat": params.Lat1,
					"lon": params.Lon1,
				},
				"to": map[string]float64{
					"lat": params.Lat2,
					"lon": params.Lon2,
				},
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func destinationTool() tool.Tool {
	return tool.NewBuilder("geo_destination").
		WithDescription("Calculate destination point given start, bearing, and distance").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat      float64 `json:"lat"`
				Lon      float64 `json:"lon"`
				Bearing  float64 `json:"bearing"`
				Distance float64 `json:"distance"`
				Unit     string  `json:"unit,omitempty"` // km (default), miles
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Convert distance to km
			distanceKm := params.Distance
			if strings.ToLower(params.Unit) == "miles" || strings.ToLower(params.Unit) == "mi" {
				distanceKm = params.Distance * 1.60934
			}

			latRad := toRadians(params.Lat)
			lonRad := toRadians(params.Lon)
			bearingRad := toRadians(params.Bearing)
			angularDist := distanceKm / earthRadiusKm

			destLat := math.Asin(math.Sin(latRad)*math.Cos(angularDist) +
				math.Cos(latRad)*math.Sin(angularDist)*math.Cos(bearingRad))
			destLon := lonRad + math.Atan2(
				math.Sin(bearingRad)*math.Sin(angularDist)*math.Cos(latRad),
				math.Cos(angularDist)-math.Sin(latRad)*math.Sin(destLat))

			result := map[string]any{
				"destination": map[string]float64{
					"lat": toDegrees(destLat),
					"lon": toDegrees(destLon),
				},
				"from": map[string]float64{
					"lat": params.Lat,
					"lon": params.Lon,
				},
				"bearing":  params.Bearing,
				"distance": params.Distance,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func midpointTool() tool.Tool {
	return tool.NewBuilder("geo_midpoint").
		WithDescription("Calculate midpoint between two coordinates").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat1 float64 `json:"lat1"`
				Lon1 float64 `json:"lon1"`
				Lat2 float64 `json:"lat2"`
				Lon2 float64 `json:"lon2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			lat1Rad := toRadians(params.Lat1)
			lon1Rad := toRadians(params.Lon1)
			lat2Rad := toRadians(params.Lat2)
			deltaLon := toRadians(params.Lon2 - params.Lon1)

			bx := math.Cos(lat2Rad) * math.Cos(deltaLon)
			by := math.Cos(lat2Rad) * math.Sin(deltaLon)

			midLat := math.Atan2(
				math.Sin(lat1Rad)+math.Sin(lat2Rad),
				math.Sqrt((math.Cos(lat1Rad)+bx)*(math.Cos(lat1Rad)+bx)+by*by))
			midLon := lon1Rad + math.Atan2(by, math.Cos(lat1Rad)+bx)

			result := map[string]any{
				"midpoint": map[string]float64{
					"lat": toDegrees(midLat),
					"lon": toDegrees(midLon),
				},
				"point1": map[string]float64{
					"lat": params.Lat1,
					"lon": params.Lon1,
				},
				"point2": map[string]float64{
					"lat": params.Lat2,
					"lon": params.Lon2,
				},
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func boundingBoxTool() tool.Tool {
	return tool.NewBuilder("geo_bounding_box").
		WithDescription("Calculate bounding box around a point").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat    float64 `json:"lat"`
				Lon    float64 `json:"lon"`
				Radius float64 `json:"radius"`
				Unit   string  `json:"unit,omitempty"` // km (default), miles
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			radiusKm := params.Radius
			if strings.ToLower(params.Unit) == "miles" || strings.ToLower(params.Unit) == "mi" {
				radiusKm = params.Radius * 1.60934
			}

			latRad := toRadians(params.Lat)
			angularDist := radiusKm / earthRadiusKm

			minLat := params.Lat - toDegrees(angularDist)
			maxLat := params.Lat + toDegrees(angularDist)

			deltaLon := toDegrees(math.Asin(math.Sin(angularDist) / math.Cos(latRad)))
			minLon := params.Lon - deltaLon
			maxLon := params.Lon + deltaLon

			result := map[string]any{
				"center": map[string]float64{
					"lat": params.Lat,
					"lon": params.Lon,
				},
				"radius": params.Radius,
				"bounds": map[string]float64{
					"min_lat": minLat,
					"max_lat": maxLat,
					"min_lon": minLon,
					"max_lon": maxLon,
				},
				"corners": []map[string]float64{
					{"lat": maxLat, "lon": minLon}, // NW
					{"lat": maxLat, "lon": maxLon}, // NE
					{"lat": minLat, "lon": maxLon}, // SE
					{"lat": minLat, "lon": minLon}, // SW
				},
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseCoordTool() tool.Tool {
	return tool.NewBuilder("geo_parse_coord").
		WithDescription("Parse coordinate string into lat/lon").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Coord string `json:"coord"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			coord := strings.TrimSpace(params.Coord)

			// Try various formats
			var lat, lon float64
			var format string
			var err error

			// Format: 40.7128, -74.0060 or 40.7128 -74.0060
			decimalPattern := regexp.MustCompile(`^(-?\d+\.?\d*)[,\s]+(-?\d+\.?\d*)$`)
			if matches := decimalPattern.FindStringSubmatch(coord); matches != nil {
				lat, err = strconv.ParseFloat(matches[1], 64)
				if err == nil {
					lon, err = strconv.ParseFloat(matches[2], 64)
				}
				if err == nil {
					format = "decimal"
				}
			}

			// Format: 40°42'46"N 74°00'22"W (DMS)
			if format == "" {
				dmsPattern := regexp.MustCompile(`(\d+)°(\d+)'(\d+\.?\d*)"?([NS])\s*(\d+)°(\d+)'(\d+\.?\d*)"?([EW])`)
				if matches := dmsPattern.FindStringSubmatch(coord); matches != nil {
					latDeg, _ := strconv.ParseFloat(matches[1], 64)
					latMin, _ := strconv.ParseFloat(matches[2], 64)
					latSec, _ := strconv.ParseFloat(matches[3], 64)
					latDir := matches[4]

					lonDeg, _ := strconv.ParseFloat(matches[5], 64)
					lonMin, _ := strconv.ParseFloat(matches[6], 64)
					lonSec, _ := strconv.ParseFloat(matches[7], 64)
					lonDir := matches[8]

					lat = latDeg + latMin/60 + latSec/3600
					if latDir == "S" {
						lat = -lat
					}

					lon = lonDeg + lonMin/60 + lonSec/3600
					if lonDir == "W" {
						lon = -lon
					}

					format = "dms"
					err = nil
				}
			}

			if format == "" {
				result := map[string]any{
					"error": "could not parse coordinate",
					"input": params.Coord,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"lat":    lat,
				"lon":    lon,
				"format": format,
				"input":  params.Coord,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatCoordTool() tool.Tool {
	return tool.NewBuilder("geo_format_coord").
		WithDescription("Format coordinates into various formats").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat    float64 `json:"lat"`
				Lon    float64 `json:"lon"`
				Format string  `json:"format,omitempty"` // decimal, dms, ddm
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			lat, lon := params.Lat, params.Lon

			// Decimal format
			decimal := map[string]float64{
				"lat": lat,
				"lon": lon,
			}

			// DMS format
			latDir := "N"
			if lat < 0 {
				latDir = "S"
				lat = -lat
			}
			lonDir := "E"
			if lon < 0 {
				lonDir = "W"
				lon = -lon
			}

			latDeg := int(lat)
			latMin := int((lat - float64(latDeg)) * 60)
			latSec := (lat - float64(latDeg) - float64(latMin)/60) * 3600

			lonDeg := int(lon)
			lonMin := int((lon - float64(lonDeg)) * 60)
			lonSec := (lon - float64(lonDeg) - float64(lonMin)/60) * 3600

			dms := map[string]string{
				"lat": strings.ReplaceAll(strings.ReplaceAll(
					strings.ReplaceAll("%d°%d'%.2f\"%s", "%d", strconv.Itoa(latDeg)),
					"%d", strconv.Itoa(latMin)),
					"%.2f", strconv.FormatFloat(latSec, 'f', 2, 64)) + latDir,
				"lon": strings.ReplaceAll(strings.ReplaceAll(
					strings.ReplaceAll("%d°%d'%.2f\"%s", "%d", strconv.Itoa(lonDeg)),
					"%d", strconv.Itoa(lonMin)),
					"%.2f", strconv.FormatFloat(lonSec, 'f', 2, 64)) + lonDir,
			}

			// DDM format (degrees decimal minutes)
			latDecMin := (params.Lat - float64(int(params.Lat))) * 60
			if params.Lat < 0 {
				latDecMin = -latDecMin
			}
			lonDecMin := (params.Lon - float64(int(params.Lon))) * 60
			if params.Lon < 0 {
				lonDecMin = -lonDecMin
			}

			ddm := map[string]string{
				"lat": strconv.Itoa(latDeg) + "°" + strconv.FormatFloat(math.Abs(latDecMin), 'f', 4, 64) + "'" + latDir,
				"lon": strconv.Itoa(lonDeg) + "°" + strconv.FormatFloat(math.Abs(lonDecMin), 'f', 4, 64) + "'" + lonDir,
			}

			result := map[string]any{
				"decimal": decimal,
				"dms":     dms,
				"ddm":     ddm,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateCoordTool() tool.Tool {
	return tool.NewBuilder("geo_validate_coord").
		WithDescription("Validate if coordinates are valid").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			validLat := params.Lat >= -90 && params.Lat <= 90
			validLon := params.Lon >= -180 && params.Lon <= 180
			valid := validLat && validLon

			var errors []string
			if !validLat {
				errors = append(errors, "latitude must be between -90 and 90")
			}
			if !validLon {
				errors = append(errors, "longitude must be between -180 and 180")
			}

			result := map[string]any{
				"valid":     valid,
				"valid_lat": validLat,
				"valid_lon": validLon,
				"lat":       params.Lat,
				"lon":       params.Lon,
			}
			if len(errors) > 0 {
				result["errors"] = errors
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func convertCoordTool() tool.Tool {
	return tool.NewBuilder("geo_convert_coord").
		WithDescription("Convert between coordinate systems").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Lat  float64 `json:"lat,omitempty"`
				Lon  float64 `json:"lon,omitempty"`
				X    float64 `json:"x,omitempty"`
				Y    float64 `json:"y,omitempty"`
				From string  `json:"from"` // wgs84, mercator
				To   string  `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			result := make(map[string]any)

			switch strings.ToLower(params.From) + "->" + strings.ToLower(params.To) {
			case "wgs84->mercator":
				// WGS84 to Web Mercator
				x := params.Lon * 20037508.34 / 180
				y := math.Log(math.Tan((90+params.Lat)*math.Pi/360)) / (math.Pi / 180)
				y = y * 20037508.34 / 180

				result["x"] = x
				result["y"] = y
				result["from"] = "wgs84"
				result["to"] = "mercator"
				result["input_lat"] = params.Lat
				result["input_lon"] = params.Lon

			case "mercator->wgs84":
				// Web Mercator to WGS84
				lon := params.X * 180 / 20037508.34
				lat := math.Atan(math.Exp(params.Y*math.Pi/20037508.34))*360/math.Pi - 90

				result["lat"] = lat
				result["lon"] = lon
				result["from"] = "mercator"
				result["to"] = "wgs84"
				result["input_x"] = params.X
				result["input_y"] = params.Y

			default:
				result["error"] = "unsupported conversion: " + params.From + " to " + params.To
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func areaPolygonTool() tool.Tool {
	return tool.NewBuilder("geo_polygon_area").
		WithDescription("Calculate area of a polygon defined by coordinates").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Points []struct {
					Lat float64 `json:"lat"`
					Lon float64 `json:"lon"`
				} `json:"points"`
				Unit string `json:"unit,omitempty"` // km2 (default), m2, mi2, acres, ha
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Points) < 3 {
				result := map[string]any{"error": "polygon requires at least 3 points"}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Use spherical excess formula for polygon area on a sphere
			n := len(params.Points)
			var sum float64

			for i := 0; i < n; i++ {
				j := (i + 1) % n
				k := (i + 2) % n

				lat1 := toRadians(params.Points[i].Lat)
				lon1 := toRadians(params.Points[i].Lon)
				lat2 := toRadians(params.Points[j].Lat)
				lon2 := toRadians(params.Points[j].Lon)
				lat3 := toRadians(params.Points[k].Lat)
				lon3 := toRadians(params.Points[k].Lon)

				// Simplified calculation using cross product method
				sum += (lon2 - lon1) * (2 + math.Sin(lat1) + math.Sin(lat2))
				_ = lat3
				_ = lon3
			}

			// More accurate: use the shoelace formula with projected coordinates
			var areaSum float64
			for i := 0; i < n; i++ {
				j := (i + 1) % n
				areaSum += toRadians(params.Points[j].Lon-params.Points[i].Lon) *
					(math.Sin(toRadians(params.Points[i].Lat)) + math.Sin(toRadians(params.Points[j].Lat)))
			}

			areaKm2 := math.Abs(areaSum * earthRadiusKm * earthRadiusKm / 2)

			var area float64
			unit := strings.ToLower(params.Unit)
			switch unit {
			case "m2", "sqm":
				area = areaKm2 * 1000000
				unit = "m2"
			case "mi2", "sqmi":
				area = areaKm2 * 0.386102
				unit = "mi2"
			case "acres", "ac":
				area = areaKm2 * 247.105
				unit = "acres"
			case "ha", "hectares":
				area = areaKm2 * 100
				unit = "ha"
			default:
				area = areaKm2
				unit = "km2"
			}

			result := map[string]any{
				"area":   area,
				"unit":   unit,
				"points": len(params.Points),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
