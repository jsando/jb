package myapp;

import com.example.lib1.Greeter;

public class Main {
    public static void main(String[] args) {
        Greeter greeter = new Greeter();
        greeter.sayHelloTo(args[0]);
    }
}