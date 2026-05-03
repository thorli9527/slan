package dev.slan.client_core_plugin;

import java.util.List;
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
}
