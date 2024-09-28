package main

import (
	"context"
	"fmt"
	"log"
	"errors"
	"os"
	"time"

	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/gookit/color.v1"
)

var collection *mongo.Collection
var ctx = context.TODO()

const (
	URI			= "mongodb://localhost:27017"
	DATABASE 	= "tasker"
	COLLECTION	= "task"
)

func init() {
	clientOptions := options.Client().ApplyURI(URI)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}

	collection = client.Database(DATABASE).Collection(COLLECTION)
}

type Task struct {
	ID        primitive.ObjectID `bson:"_id"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
	Text      string             `bson:"text"`
	Completed bool               `bson:"completed"`
}

func main() {
	app := &cli.App{
		Name: "tasker",
		Usage: "A simple CLI program to manage your tasks",
		Action: func(ctx *cli.Context) error {
			tasks, err := getPending()
			if err != nil {
				if err == mongo.ErrNoDocuments {
					fmt.Println("Nothing to see here.\nRun `add 'task'` to add a task")
					return nil
				}
				return err
			}
			printTasks(tasks)
			return nil
		},
		Commands: []*cli.Command{
			{
				Name: "add",
				Aliases: []string{"a"},
				Usage: "Add new task to the list",
				Action: func(ctx *cli.Context) error {
					str := ctx.Args().First()
					if str == "" {
						return errors.New("can not add empty task")
					}

					task := &Task{
						ID: primitive.NewObjectID(),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						Text: str,
						Completed: false,
					}

					return createNewTask(task)
				},
			},
			{
				Name: "all",
				Aliases: []string{"l"},
				Usage: "List all the tasks",
				Action: func(ctx *cli.Context) error {
					tasks, err := getAll()

					if err != nil {
						if err == mongo.ErrNoDocuments {
							fmt.Println("Nothing to see here.\nRun `add 'task'` to add a task")
							return nil
						}
						return err
					}

					printTasks(tasks)
					return nil
				},
			},
			{
				Name: "done",
				Aliases: []string{"d"},
				Usage: "complete a task on the list",
				Action: func(ctx *cli.Context) error {
					text := ctx.Args().First()
					if text == "" {
						return errors.New("Task Name is mandatory\n Run `all` to get list of all tasks")
					}
					return completeTask(text)
				},
			},
			{
				Name: "finished",
				Aliases: []string{"f"},
				Usage: "list completed tasks",
				Action: func(ctx *cli.Context) error {
					tasks, err := getFinished()
					if err != nil {
						if err == mongo.ErrNoDocuments {
							fmt.Print("Nothing to see here.\nRun `done 'task'` to complete a task")
							return nil
						}

						return err
					}

					printTasks(tasks)
					return nil
				},
			},
			{
				Name: "rm",
				Usage: "deletes a task on the list",
				Action: func(ctx *cli.Context) error {
					text := ctx.Args().First()
					if text == "" {
						return errors.New("Task Name is mandatory\n Run `all` to get list of all tasks")
					}
					err := deleteTask(text)
					if err != nil {
						return err
					}

					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func createNewTask(task *Task) error {
	_, err := collection.InsertOne(ctx, task)
	return err
}

func getAll() ([]*Task, error) {
	filter := bson.D{{}}
	return filterTasks(filter)
}


func printTasks(tasks []*Task) {
	for i, v := range tasks {
		if v.Completed {
			color.Green.Printf("%d: %s\n", i+1, v.Text)
		} else {
			color.Yellow.Printf("%d: %s\n", i+1, v.Text)
		}
	}
}

func completeTask(text string) error {
	filter := bson.D{primitive.E{Key: "text", Value: text}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "completed", Value: true},
		primitive.E{Key: "updated_at", Value: time.Now()},
	}}}

	tasks, err := filterTasks(filter)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("no task found to mark as complete\nrun `all` to get all tasks")
		}
		return err
	}

	for _, v := range tasks {
		if v.Completed {
			log.Println("Task is already marked as completed")
		} else {
			t := &Task{}
			return collection.FindOneAndUpdate(ctx, filter, update).Decode(t)
		}
	}
	return nil
}

func getPending() ([]*Task, error) {
	filter := bson.D{primitive.E{Key: "completed", Value: false}}
	return filterTasks(filter)
}


func getFinished() ([]*Task, error) {
	filter := bson.D{primitive.E{Key: "completed", Value: true}}
	return filterTasks(filter)
}


func filterTasks(filter interface{}) ([]*Task, error) {
	// A slice of tasks for storing the decoded documents
	var tasks []*Task

	cur, err := collection.Find(ctx, filter)
	if err != nil {
		return tasks, err
	}

	// Iterate through the cursor and decode each document one at a time
	for cur.Next(ctx) {
		var t Task
		err := cur.Decode(&t)
		if err != nil {
			return tasks, err
		}

		tasks = append(tasks, &t)
	}

	if err := cur.Err(); err != nil {
		return tasks, err
	}

	// once exhausted, close the cursor
	cur.Close(ctx)

	if len(tasks) == 0 {
		return tasks, mongo.ErrNoDocuments
	}

	return tasks, nil
}

func deleteTask(text string) error {
	filter := bson.D{primitive.E{Key: "text", Value: text}}

	tasks, err := filterTasks(filter)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("no task found to delete\nrun `all` to get all tasks")
		}
		return err
	}

	for _, v := range tasks {
		res, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			log.Printf("Failed to delete task %s",  v.Text)
		}

		if res.DeletedCount == 0 {
			return errors.New("no tasks were deleted")
		}
	}
	return nil
}

