import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.common.SimplePlugin;
import io.temporal.common.interceptors.WorkerInterceptorBase;
import io.temporal.common.interceptors.WorkflowClientInterceptorBase;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

class PluginsSnippet {

  // @@@SNIPSTART java-plugin-activity
  @ActivityInterface
  public interface SomeActivity {
    @ActivityMethod
    void someActivity();
  }

  public class SomeActivityImpl implements SomeActivity {
    @Override
    public void someActivity() {
      // Activity implementation
    }
  }

  SimplePlugin activityPlugin =
      SimplePlugin.newBuilder("PluginName")
          .registerActivitiesImplementations(new SomeActivityImpl())
          .build();
  // @@@SNIPEND

  // @@@SNIPSTART java-plugin-workflow
  @WorkflowInterface
  public interface HelloWorkflow {
    @WorkflowMethod
    String run(String name);
  }

  public static class HelloWorkflowImpl implements HelloWorkflow {
    @Override
    public String run(String name) {
      return "Hello, " + name + "!";
    }
  }

  SimplePlugin workflowPlugin =
      SimplePlugin.newBuilder("PluginName")
          .registerWorkflowImplementationTypes(HelloWorkflowImpl.class)
          .build();
  // @@@SNIPEND

  // @@@SNIPSTART java-plugin-nexus
  // Example Nexus service implementation
  public class WeatherService {
    public Weather getWeather(WeatherInput input) {
      return new Weather(input.getCity(), "14-20C", "Sunny with wind.");
    }
  }

  public static class Weather {
    private final String city;
    private final String temperatureRange;
    private final String conditions;

    public Weather(String city, String temperatureRange, String conditions) {
      this.city = city;
      this.temperatureRange = temperatureRange;
      this.conditions = conditions;
    }

    // Getters...
  }

  public static class WeatherInput {
    private final String city;

    public WeatherInput(String city) {
      this.city = city;
    }

    public String getCity() {
      return city;
    }
  }

  SimplePlugin nexusPlugin =
      SimplePlugin.newBuilder("PluginName")
          .registerNexusServiceImplementation(new WeatherService())
          .build();
  // @@@SNIPEND

  // @@@SNIPSTART java-plugin-converter
  SimplePlugin converterPlugin =
      SimplePlugin.newBuilder("PluginName")
          .customizeDataConverter(
              existingConverter -> {
                // Customize the data converter
                // This example keeps the existing converter unchanged
                // In practice, you might wrap it with additional functionality
                return existingConverter;
              })
          .build();
  // @@@SNIPEND

  // @@@SNIPSTART java-plugin-interceptors
  public class SomeWorkerInterceptor extends WorkerInterceptorBase {
    // Your worker interceptor implementation
  }

  public class SomeClientInterceptor extends WorkflowClientInterceptorBase {
    // Your client interceptor implementation
  }

  SimplePlugin interceptorPlugin =
      SimplePlugin.newBuilder("PluginName")
          .addWorkerInterceptors(new SomeWorkerInterceptor())
          .addClientInterceptors(new SomeClientInterceptor())
          .build();
  // @@@SNIPEND
}
