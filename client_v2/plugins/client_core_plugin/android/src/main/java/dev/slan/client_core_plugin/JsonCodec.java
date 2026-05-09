package dev.slan.client_core_plugin;

import java.util.List;
import java.util.HashMap;
import java.util.ArrayList;
import java.util.Map;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

final class JsonCodec {
  private JsonCodec() {}

  static JSONObject toJsonObject(Object value) throws JSONException {
    if (value instanceof JSONObject) {
      return (JSONObject) value;
    }
    if (value instanceof Map<?, ?>) {
      JSONObject object = new JSONObject();
      for (Map.Entry<?, ?> entry : ((Map<?, ?>) value).entrySet()) {
        if (entry.getKey() != null) {
          object.put(String.valueOf(entry.getKey()), toJsonValue(entry.getValue()));
        }
      }
      return object;
    }
    return new JSONObject();
  }

  static Map<String, Object> toMap(String json) throws JSONException {
    return toMap(new JSONObject(json == null || json.trim().isEmpty() ? "{}" : json));
  }

  static Map<String, Object> toMap(JSONObject object) throws JSONException {
    Map<String, Object> out = new HashMap<>();
    if (object == null) {
      return out;
    }
    JSONArray names = object.names();
    if (names == null) {
      return out;
    }
    for (int index = 0; index < names.length(); index += 1) {
      String name = names.optString(index, "");
      if (!name.isEmpty()) {
        out.put(name, fromJsonValue(object.opt(name)));
      }
    }
    return out;
  }

  private static Object toJsonValue(Object value) throws JSONException {
    if (value == null) {
      return JSONObject.NULL;
    }
    if (value instanceof String || value instanceof Number || value instanceof Boolean) {
      return value;
    }
    if (value instanceof Map<?, ?>) {
      return toJsonObject(value);
    }
    if (value instanceof List<?>) {
      JSONArray array = new JSONArray();
      for (Object item : (List<?>) value) {
        array.put(toJsonValue(item));
      }
      return array;
    }
    return String.valueOf(value);
  }

  private static Object fromJsonValue(Object value) throws JSONException {
    if (value == null || value == JSONObject.NULL) {
      return null;
    }
    if (value instanceof String || value instanceof Number || value instanceof Boolean) {
      return value;
    }
    if (value instanceof JSONObject) {
      return toMap((JSONObject) value);
    }
    if (value instanceof JSONArray) {
      JSONArray array = (JSONArray) value;
      List<Object> out = new ArrayList<>();
      for (int index = 0; index < array.length(); index += 1) {
        out.add(fromJsonValue(array.opt(index)));
      }
      return out;
    }
    return String.valueOf(value);
  }
}
