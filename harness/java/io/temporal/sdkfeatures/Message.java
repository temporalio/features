package io.temporal.sdkfeatures;

import java.util.Objects;

public class Message {
  private boolean spec;

  public Message() {}

  public Message(boolean spec) {
    this.spec = spec;
  }

  public boolean getSpec() {
    return spec;
  }

  public void setSpec(boolean spec) {
    this.spec = spec;
  }

  @Override
  public boolean equals(Object o) {
    if (this == o) return true;
    if (o == null || getClass() != o.getClass()) return false;
    Message message = (Message) o;
    return getSpec() == message.getSpec();
  }

  @Override
  public int hashCode() {
    return Objects.hash(getSpec());
  }

  @Override
  public String toString() {
    return "Message{spec=" + spec + "}";
  }
}
;
