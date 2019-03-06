package main

import (
  "os"
  "fmt"
  "log"
  "encoding/json"
  "net/http"
  "github.com/go-redis/redis"
)

var RedisClient = redis.NewClient(&redis.Options{
  Addr:     "localhost:6379",
  Password: "", // no password set
  DB:       15,  // use default DB
})

func main() {
  if os.Getenv("GCOOL_REDIS_PASS") == "" {
    fmt.Println("Undefined env variable GCOOL_REDIS_PASS")
    os.Exit(1)
  }

  if os.Getenv("GCOOL_QUIZ_QUES_PATH") == "" {
    fmt.Println("Undefined env variable GCOOL_QUIZ_QUES_PATH")
    os.Exit(1)
  }

  http.HandleFunc("/", ex_json)                       // GET

  http.HandleFunc("/api/v1/activate", mng_quiz)       // POST
  http.HandleFunc("/api/v1/start",    mng_quiz)       // POST
  http.HandleFunc("/api/v1/finish",   mng_quiz)       // POST
  http.HandleFunc("/api/v1/destroy",  mng_quiz)       // POST

  http.HandleFunc("/api/v1/status", chkQuizStatus)    // GET

  http.HandleFunc("/api/v1/join", join_quiz)          // POST
  http.HandleFunc("/api/v1/record", rec_response)     // POST


  if len(os.Args) > 1 {
    fmt.Println(os.Args)
    RedisClient = redis.NewClient(&redis.Options{
      Addr:     os.Args[1] + ":6379",
      Password: os.Getenv("GCOOL_REDIS_PASS"), // no password set
      DB:       15,  // use default DB
    })    
  }

  pong, err := RedisClient.Ping().Result()
  fmt.Println("Redis", pong, err)
  if err != nil {
    return
  }

  fmt.Println("Ready...")
  log.Fatal(http.ListenAndServe(":8080", nil))
}

////////////////////////////////////////////////////////////////////////////
//----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| //
////////////////////////////////////////////////////////////////////////////
type mngQuizReqData struct {
  Code    uint        `json:"code"`
  Action  string      `json:"action"`
}

type mngQuizRespData struct {
  Id      string      `json:"id"`
  Error   string      `json:"error"`
}

func mng_quiz(w http.ResponseWriter, r *http.Request) {

  var data mngQuizReqData

  decoder := json.NewDecoder(r.Body)

  err := decoder.Decode(&data)
  if err != nil {
    panic(err)
  }

  code_s := fmt.Sprintf("%d",data.Code)
  fmt.Println("MANAGE QUIZ: " + code_s + " | " + data.Action)

  w.Header().Set("Content-Type", "application/json")
  if data.Code % 10000 != 0 && data.Code < 50000 {
    w.WriteHeader(http.StatusNotFound)
    return
  }

  quiz_id := "quiz_" + code_s
  mng_quiz_resp := mngQuizRespData {}

  switch data.Action {

  case "activate":
    if activateQuiz(quiz_id) == false {
      w.WriteHeader(http.StatusUnprocessableEntity)
      mng_quiz_resp.Error = "Already activated!"
    } else {
      resetQuizData(quiz_id)

      w.WriteHeader(http.StatusCreated)
      mng_quiz_resp.Id = quiz_id
    }

  case "start":

    if startQuiz(quiz_id) == false {
      w.WriteHeader(http.StatusUnprocessableEntity)
      mng_quiz_resp.Error = "Already started!"
    } else {
      w.WriteHeader(http.StatusCreated)
      mng_quiz_resp.Id = quiz_id
    }
    
  case "finish":
    finishQuiz(quiz_id)

  case "destroy":
    destroyQuiz(quiz_id)
    resetQuizData(quiz_id)
    
  default:
    panic("unrecognized escape character")
  }

  js, err := json.Marshal(mng_quiz_resp)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
  w.Write(js)
}

////////////////////////////////////////////////////////////////////////////
func activateQuiz(quiz_id string) bool {
  val, err := RedisClient.HSetNX(quiz_id, "active", "1").Result()
  if err == redis.Nil {
    fmt.Println("Quiz " + quiz_id + " does not exist")
    return false
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("activateQuiz", quiz_id, val)
  }

  return val
}

func destroyQuiz(quiz_id  string) int64 {
  val, err := RedisClient.Del(quiz_id).Result()
  if err == redis.Nil {
    fmt.Println(quiz_id + "does not exist")
    return 0
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("destroyQuiz", quiz_id, val)
  }

  return val
}

func startQuiz(quiz_id  string) bool {
  val, err := RedisClient.HSetNX(quiz_id, "start", "1").Result()
  if err == redis.Nil {
    fmt.Println("Quiz " + quiz_id + " does not exist")
    return false
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("startQuiz", quiz_id, val)
  }

  return val
}

func finishQuiz(quiz_id  string) int64 {
  val, err := RedisClient.HDel(quiz_id, "start").Result()
  if err == redis.Nil {
    fmt.Println("Quiz " + quiz_id + " does not exist")
    return 0
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("finishQuiz", quiz_id, val)
  }

  return val
}

func isQuizStarted(quiz_id  string) bool {
  val, err := RedisClient.HGet(quiz_id, "start").Result()
  if err == redis.Nil {
    fmt.Println("Quiz " + quiz_id + " not started yet")
    return false
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("isQuizStarted", quiz_id, val)
  }

  return val == "1"
}

func isQuizActive(quiz_id  string) bool {
  val, err := RedisClient.HGet(quiz_id, "active").Result()
  if err == redis.Nil {
    fmt.Println("Quiz " + quiz_id + " not Active yet")
    return false
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("isQuizActive", quiz_id, val)
  }

  return val == "1"
}

////////////////////////////////////////////////////////////////////////////
//----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| //
////////////////////////////////////////////////////////////////////////////
type joinReqData struct {
  Code int
  Name string
}

type joinRespData struct {
  Id        string        `json:"id"`
  Session   string        `json:"session"`
  Qurl      string        `json:"qurl"`
}

func join_quiz(w http.ResponseWriter, r *http.Request) {

  var data joinReqData

  decoder := json.NewDecoder(r.Body)

  err := decoder.Decode(&data)
  if err != nil {
    panic(err)
  }

  code_s := fmt.Sprintf("%d",data.Code)
  fmt.Println("JOIN: " + code_s + " | " + data.Name)

  w.Header().Set("Content-Type", "application/json")
  if data.Code % 10000 != 0 && data.Code < 50000 {
    w.WriteHeader(http.StatusNotFound)
    return
  }

  w.WriteHeader(http.StatusCreated)

  quiz_id       := "quiz_" + code_s
  session_id    := fmt.Sprintf("session_id_%d", data.Code * 1232)
  join_resp := joinRespData {
                  quiz_id, 
                  session_id,
                  os.Getenv("GCOOL_QUIZ_QUES_PATH") + quiz_id + "/questions.json"}

  js, err := json.Marshal(join_resp)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
  w.Write(js)

  // Add student name to the ongoing quiz session
  add_student_to_quiz(quiz_id, session_id, data.Name)
}

////////////////////////////////////////////////////////////////////////////
//----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| //
////////////////////////////////////////////////////////////////////////////
type chkQuizReqData struct {
  Id          string   `json:"id"`
}

type chkQuizRespData struct {
  IsActive   bool       `json:"is_active"`
  IsStarted  bool       `json:"is_started"`
  Players    []string   `json:"players"`
}

func chkQuizStatus(w http.ResponseWriter, r *http.Request) {

  var in_data chkQuizReqData

  decoder := json.NewDecoder(r.Body)

  err := decoder.Decode(&in_data)
  if err != nil {
    panic(err)
  }

  out_data := chkQuizRespData{isQuizActive(in_data.Id), 
    isQuizStarted(in_data.Id),
    getPlayers(in_data.Id)}

  js, err := json.Marshal(out_data)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }

  w.Header().Set("Content-Type", "application/json")
  w.Write(js)
}

////////////////////////////////////////////////////////////////////////////
//----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| //
////////////////////////////////////////////////////////////////////////////
type rrReqData struct {
  Id          string    `json:"id"`
  Session     string    `json:"session"`
  QuesId      string    `json:"ques_id"`
  Name        string    `json:"name"`
  IsCorrect   bool      `json:"is_correct"`
  TimeTaken   uint      `json:"time_taken"`
}

type ldrBoardEntry struct {
  Name        string    `json:"name"`
  Score       int       `json:"score"`
  Rank        uint      `json:"rank"`
}

type rrRespData struct {
  Id          string          `json:"id"`
  Session     string          `json:"session"`
  QuesId      string          `json:"ques_id"`
  PlayerCount int64            `json:"player_count"`
  Ldrboard    []ldrBoardEntry `json:"leaderboard"`
}

func rec_response(w http.ResponseWriter, r *http.Request) {

  ////////////////////// INPUT //////////////////////
  var data rrReqData

  decoder := json.NewDecoder(r.Body)

  err := decoder.Decode(&data)
  if err != nil {
    panic(err)
  }

  fmt.Println("RecordResponse: " + data.Id + " | " + data.Name + " | " + data.QuesId)

  qz_key := getQuizDataKey(data.Id)

  // Update student score
  if data.IsCorrect == true {
    score_step := 10000 - data.TimeTaken + 1000
    RedisClient.ZIncrBy(qz_key, float64(score_step), data.Name)
  }

  ////////////////////// OUTPUT //////////////////////
  ldrboard := []ldrBoardEntry {}

  leaderboard_raw := RedisClient.ZRevRangeWithScores(qz_key, 0, 100000000).Val()
  fmt.Println("Leaderboard", leaderboard_raw)

  for cur_index, entry := range leaderboard_raw {
    name, _ := entry.Member.(string)
    ldrboard = append(ldrboard, ldrBoardEntry{name, int(entry.Score), uint(cur_index + 1)})
  }

  output := rrRespData{data.Id, data.Session, data.QuesId, RedisClient.ZCard(qz_key).Val(), ldrboard}
  js, err := json.Marshal(output)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }

  w.Header().Set("Content-Type", "application/json")
  w.Write(js)
}

////////////////////////////////////////////////////////////////////////////
//----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| //
////////////////////////////////////////////////////////////////////////////
func add_student_to_quiz(quiz_id string, session_id string, name string) {
  fmt.Printf("Adding %s to quiz %s of session_id %s\n", name, quiz_id, session_id)

  qz_key := getQuizDataKey(quiz_id)

  RedisClient.ZAdd(qz_key, redis.Z{
    Score:  float64(0),
    Member: name,
  })
}

func getQuizDataKey(quiz_id string) string {
  return fmt.Sprintf("Quiz:%s:playerData", quiz_id)
}

func resetQuizData(quiz_id string) int64 {  
  qz_key := getQuizDataKey(quiz_id)

  val, err := RedisClient.Del(qz_key).Result()
  if err == redis.Nil {
    fmt.Println(quiz_id + "does not exist")
    return 0
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("resetQuizData", quiz_id, val)
  }
  return val
}

func getPlayers(quiz_id string) []string {
  qz_key := getQuizDataKey(quiz_id)

  val, err := RedisClient.ZRange(qz_key, 0, -1).Result()
  if err == redis.Nil {
    fmt.Println(quiz_id + "does not exist")
    return []string{}
  } else if err != nil {
    panic(err)
  } else {
    fmt.Println("getPlayers", quiz_id, val)
  }

  return val
}

////////////////////////////////////////////////////////////////////////////
//----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| ----- ||||| //
////////////////////////////////////////////////////////////////////////////

type Profile struct {
  Name    string
  Hobbies []string
}

func ex_json(w http.ResponseWriter, r *http.Request) {
  profile := Profile{"Something", []string{"IS", "WRONG!"}}

  js, err := json.Marshal(profile)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }

  w.Header().Set("Content-Type", "application/json")
  w.Write(js)
}