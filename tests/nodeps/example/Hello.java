package example;

public class Hello {
    public static void main(String[] args) {
        var name = "world";
        if (args.length > 0) {
            name = args[0];
        }
        System.out.println("Hello," + name);
    }
}