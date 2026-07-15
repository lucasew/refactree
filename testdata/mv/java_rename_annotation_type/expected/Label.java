package demo;

import java.lang.annotation.*;

@Retention(RetentionPolicy.RUNTIME)
public @interface Label {
  String value() default "";
}
