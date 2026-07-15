package demo;

import java.lang.annotation.*;

@Retention(RetentionPolicy.RUNTIME)
public @interface Tag {
  String value() default "";
}
