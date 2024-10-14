package example;

public class Greeting {
    
    private final String name;

    public Greeting(String name) {
        this.name = name;
    }

    public void greet(String greeting) {
        System.out.println(greeting + this.name);
    }
}